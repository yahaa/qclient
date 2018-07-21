package qclient

import (
    "fmt"
    "io"
    "os"
    "bytes"
    "time"
    "sync"
    "path"
    "net/http"
    "io/ioutil"
    "context"
    "strings"
    "errors"
    "strconv"

    "github.com/qiniu/api.v7/auth/qbox"
    "github.com/qiniu/api.v7/storage"
)

const (
    retBody = `{"key":"$(key)","hash":"$(etag)","fsize":$(fsize),"bucket":"$(bucket)","name":"$(x:name)"}`
)

// QStat 描述上传文件后的操作状态
type QStat struct {
    Key    string
    Hash   string
    FSize  int
    Bucket string
    Name   string
}

// OpStat 描述对文件管理的操作状态
type OpStat struct {
    Status  string
    Message string
}

// FileInfo 描述文件的详细信息
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

// QClient 七牛对象存储客户端
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

// trimPath path 路径修剪
func (q *QClient) trimPath(path string, item storage.ListItem) (*FileInfo, bool) {

    fi := FileInfo{
        PutTime: item.PutTime,
    }

    if strings.LastIndex(item.Key, path) == 0 {
        path = strings.Trim(path, "/ ")
        key := strings.Trim(item.Key, "/ ")
        spl1 := strings.Split(path, "/")
        spl2 := strings.Split(key, "/")

        ok := true
        for i, v := range spl1 {
            if spl2[i] != v {
                ok = false
                break
            }
        }

        n1 := len(spl1)
        n2 := len(spl2)

        if ok && n2 > n1 {
            fi.Name = spl2[n1]

            if n2-n1 >= 2 {
                fi.IsDir = true
            } else {
                fi.IsDir = false
                fi.Key = item.Key
                fi.Hash = item.Hash
                fi.Type = item.Type
                fi.MimeType = item.MimeType
                fi.Size = item.Fsize

            }
            return &fi, true
        }

    }
    return nil, false

}

// Delete 单个删除
func (q *QClient) delete(path string) error {
    deleteOps := make([]string, 0)
    deleteOps = append(deleteOps, storage.URIDelete(q.bucket, path))

    _, err := q.bktManager.Batch(deleteOps)
    if err != nil {
        return err
    }
    return nil
}

func (q *QClient) listItem(path string) []storage.ListItem {
    var marker string
    items := make([]storage.ListItem, 0)
    for {
        entries, _, nextMar, hasNext, err := q.bktManager.ListFiles(q.bucket, path, "", marker, 1000)
        if err != nil {
            return items
        }

        items = append(items, entries...)

        if hasNext {
            marker = nextMar
        } else {
            break
        }
    }
    return items

}

// URLFor 生成公开连接地址
func (q *QClient) URLFor(path string) string {
    deadline := time.Now().Add(time.Hour * 3).Unix()
    return storage.MakePrivateURL(q.mac, q.domain, path, deadline)
}

// Writer 负责把数据写入到七牛云存储
func (q *QClient) Writer(path string, reader io.ReaderAt, fSize int64) (*QStat, error) {

    policy := &storage.PutPolicy{
        Scope:       q.bucket,
        CallbackURL: q.cbURL,
        ReturnBody:  retBody,
    }
    ret := QStat{}
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

// Push 用以把 data 推送到指定 path 下,path 即是data 数据在七牛云存储的 key
func (q *QClient) Push(path string, data []byte) (*QStat, error) {
    stat, err := q.Writer(path, bytes.NewReader(data), int64(len(data)))
    return stat, err
}

// PushFile 推送指定文件到七牛云
func (q *QClient) PushFile(filename string) (*QStat, error) {
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
func (q *QClient) PushR(path string) []QStat {

    stats := make([]QStat, 0)
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

// Reader 从七牛云存储中中获取一个可读的 reader ，用以从其中读取数据
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

// Pull 拉取文件
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

// List 模仿Linux/unix ls 命令列举 path 下的所有文件,
func (q *QClient) List(path string) []FileInfo {
    filter := make(map[string]bool)

    fis := make([]FileInfo, 0)

    items := q.listItem(path)

    for _, item := range items {
        fi, ok := q.trimPath(path, item)
        exist := filter[fi.Name]
        if !ok || exist {
            continue
        }
        filter[fi.Name] = true
        fis = append(fis, *fi)
    }

    return fis
}

// Delete 删除一个或者多个文件，支持按前缀删除
func (q *QClient) Delete(paths ...string) []OpStat {
    const (
        ok  = "success"
        bad = "error"
    )
    opStat := make([]OpStat, 0)
    for _, path := range paths {
        for _, v := range q.listItem(path) {
            err := q.delete(v.Key)

            if err == nil {
                opStat = append(opStat, OpStat{ok, fmt.Sprintf("delete %s %s", v.Key, ok)})
            } else {
                opStat = append(opStat, OpStat{bad, fmt.Sprintf("delete %s %s ", v.Key, bad)})
            }
        }
    }

    return opStat

}
