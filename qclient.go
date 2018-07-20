package qclient

import (
    "context"
    "io"
    "os"
    "bytes"
    "time"
    "net/http"
    "io/ioutil"
    "github.com/qiniu/api.v7/auth/qbox"
    "github.com/qiniu/api.v7/storage"
    "strings"
    "errors"
    "sync"
    "path"
    "strconv"
)

const (
    retBody = `{"key":"$(key)","hash":"$(etag)","fsize":$(fsize),"bucket":"$(bucket)","name":"$(x:name)"}`
)

type Stat struct {
    Key    string
    Hash   string
    FSize  int
    Bucket string
    Name   string
}

type FileInfo struct {
    Key      string `json:"key"`
    Hash     string `json:"hash"`
    Size     int64  `json:"size"`
    PutTime  int64  `json:"putTime"`
    MimeType string `json:"mimeType"`
    Type     int    `json:"type"`
    IsDir    bool   `json:"isDir"`
    Name     string `json:"name"`
}

type QClient struct {
    accessKey  string
    secretKey  string
    bucket     string
    domain     string
    endpoint   string
    config     *storage.Config
    cbURL      string
    mac        *qbox.Mac
    bktManager *storage.BucketManager
    uploader   *storage.ResumeUploader
}

// cbURL=args[0],endpoint=args[1]
func NewQClient(ak, sk, bucket, domain string, useHttps, useCDN bool, args ...string) (*QClient) {

    mac := qbox.NewMac(ak, sk)
    var (
        cbURL    string
        endpoint string
    )
    switch len(args) {
    case 2:
        cbURL, endpoint = args[0], args[1]
    case 1:
        cbURL = args[0]
    }

    cfg := storage.Config{
        UseHTTPS:      useHttps,
        UseCdnDomains: useCDN,
    }
    bktManager := storage.NewBucketManager(mac, &cfg)
    uploader := storage.NewResumeUploader(&cfg)

    return &QClient{
        accessKey:  ak,
        secretKey:  sk,
        bucket:     bucket,
        domain:     domain,
        endpoint:   endpoint,
        config:     &cfg,
        mac:        mac,
        cbURL:      cbURL,
        bktManager: bktManager,
        uploader:   uploader,
    }

}

// extra 用于构造上传到七牛云存储上的扩展名
func (q *QClient) extra(kind string) storage.RputExtra {
    return storage.RputExtra{
        Params: map[string]string{
            "x:name": kind,
        },
    }
}

// qPath 保证文件一定有前缀 "/"
func (q *QClient) qPath(path string) string {
    if !strings.HasPrefix(path, "/") {
        path = "/" + path
    }
    return path

}

// pathBase 获取不包含路径的文件名
func (q *QClient) pathBase(fName string) string {
    return path.Base(fName)

}

func (q *QClient) URLFor(path string) string {
    deadline := time.Now().Add(time.Hour * 3).Unix()
    return storage.MakePrivateURL(q.mac, q.domain, path, deadline)
}

// Writer 负责把数据写入到七牛云存储
func (q *QClient) Writer(path string, reader io.ReaderAt, fSize int64) (*Stat, error) {

    policy := &storage.PutPolicy{
        Scope:       q.bucket,
        CallbackURL: q.cbURL,
        ReturnBody:  retBody,
    }
    ret := Stat{}
    extra := q.extra(path)
    err := q.uploader.Put(
        context.Background(),
        &ret,
        policy.UploadToken(q.mac),
        q.qPath(path),
        reader,
        fSize,
        &extra,
    )
    return &ret, err
}

func (q *QClient) Push(path string, data []byte) (*Stat, error) {
    stat, err := q.Writer(path, bytes.NewReader(data), int64(len(data)))
    return stat, err
}

func (q *QClient) PushFile(filename string) (*Stat, error) {
    f, err := os.Open(filename)
    if err != nil {
        return nil, err
    }

    info, err := f.Stat()
    if err != nil {
        return nil, err
    }
    ret, err := q.Writer(filename, f, info.Size())
    return ret, err
}

//PushR 递归上传
func (q *QClient) PushR(path string) []Stat {

    stats := make([]Stat, 0)
    var wg sync.WaitGroup

    worker := func(fn string) {
        wg.Add(1)
        stat, err := q.PushFile(fn)
        if err != nil {
            return
        }
        stats = append(stats, *stat)
        defer wg.Done()

    }

    var dfs func(dir string)
    dfs = func(dir string) {

        fs, err := ioutil.ReadDir(dir)
        if err != nil {
            return
        }

        for _, f := range fs {
            if f.IsDir() {
                dfs(dir + "/" + f.Name())
            } else {
                fName := dir + "/" + f.Name()
                go worker(fName)
            }
        }
    }

    dfs(path)
    wg.Wait()

    return stats

}

func (q *QClient) Reader(path string, offset int64) (io.ReadCloser, error) {
    url := q.URLFor(path)
    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return nil, err
    }
    if offset > 0 {
        req.Header.Add("Range", "bytes="+strconv.FormatInt(offset, 10)+"-")
    }

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, err
    }
    if resp.StatusCode/100 != 2 {
        return nil, errors.New(resp.Status)
    }
    return resp.Body, err
}

func (q *QClient) Pull(path string) ([]byte, error) {
    reader, err := q.Reader(path, 0)
    if err != nil {
        return nil, err
    }
    defer reader.Close()
    return ioutil.ReadAll(reader)
}

// PullAndSave 下载文件并保存到指定目录,dst="/aa/cc"
func (q *QClient) PullTo(path, dst string) error {
    reader, err := q.Reader(path, 0)
    if err != nil {
        return err
    }
    defer reader.Close()

    err = os.MkdirAll(dst, os.ModePerm)
    if err != nil {
        return err
    }

    fName := q.pathBase(path)

    f, err := os.Create(dst + fName)
    if err != nil {
        return err
    }
    defer f.Close()

    _, err = io.Copy(f, reader)
    return err
}

func (q *QClient) List(path string) []FileInfo {
    fis := make([]FileInfo, 0)
    var marker string
    for {
        entries, _, nextMar, hasNext, err := q.bktManager.ListFiles(q.bucket, path, "", marker, 1000)
        if err != nil {
            break
        }

        for _, en := range entries {
            fi, ok := q.detail(path, &en)
            if !ok {
                continue
            }
            fis = append(fis, *fi)
        }

        if hasNext {
            marker = nextMar
        } else {
            break
        }
    }
    return fis
}

func (q *QClient) detail(path string, item *storage.ListItem) (*FileInfo, bool) {

    fi := FileInfo{
        Key:      item.Key,
        Hash:     item.Hash,
        Size:     item.Fsize,
        PutTime:  item.PutTime,
        MimeType: item.MimeType,
        Type:     item.Type,
    }

    name := strings.Replace(item.Key, path, "", 1)
    if len(name) <= 0 {
        return nil, false
    }

    spl := strings.Split(name, "/")
    if len(spl) <= 0 {
        return nil, false
    }

    if len(spl) >= 2 {
        fi.IsDir = true
        if len(spl[0]) == 0 {
            fi.Name = spl[1]
        } else {
            fi.Name = spl[0]
        }
    } else {
        fi.IsDir = false
        if len(spl[0]) == 0 {
            return nil, false
        } else {
            fi.Name = spl[0]
        }
    }
    return &fi, true

}
