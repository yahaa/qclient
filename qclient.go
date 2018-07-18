package qclient

import (
    "github.com/qiniu/api.v7/auth/qbox"
    "github.com/qiniu/api.v7/storage"
    "context"
    "io"
    "os"
    "bytes"
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

type QClient struct {
    accessKey  string
    secretKey  string
    bucket     string
    domain     string
    endpoint   string
    config     *storage.Config
    mac        *qbox.Mac
    putPolicy  *storage.PutPolicy
    uploader   *storage.ResumeUploader
    bktManager *storage.BucketManager
}

// cbURL=args[0],endpoint=args[1]
func NewQClient(ak, sk, bucket, domain string, useHttps, useCDN bool, args ...string) (*QClient) {
    mac := qbox.NewMac(ak, sk)
    var cbURL, endpoint string
    switch len(args) {
    case 2:
        cbURL, endpoint = args[0], args[1]
    case 1:
        cbURL = args[0]
    }

    putPolicy := &storage.PutPolicy{
        Scope:       bucket,
        CallbackURL: cbURL,
        ReturnBody:  retBody,
    }
    cfg := storage.Config{
        UseHTTPS:      useHttps,
        UseCdnDomains: useCDN,
    }
    uploader := storage.NewResumeUploader(&cfg)
    bktManager := storage.NewBucketManager(mac, &cfg)

    return &QClient{
        accessKey:  ak,
        secretKey:  sk,
        bucket:     bucket,
        domain:     domain,
        endpoint:   endpoint,
        config:     &cfg,
        mac:        mac,
        putPolicy:  putPolicy,
        uploader:   uploader,
        bktManager: bktManager,
    }
}

func (q *QClient) upToken() string {
    return q.putPolicy.UploadToken(q.mac)
}

func (q *QClient) extra(kind string) storage.RputExtra {
    return storage.RputExtra{
        Params: map[string]string{
            "x:name": kind,
        },
    }
}

func (q *QClient) put(path string, reader io.ReaderAt, fSize int64) (*Stat, error) {
    ret := Stat{}
    extra := q.extra(path)
    err := q.uploader.Put(context.Background(), &ret, q.upToken(), path, reader, fSize, &extra)
    return &ret, err
}

func (q *QClient) PutFile(filename string) (*Stat, error) {
    f, err := os.Open(filename)
    if err != nil {
        return nil, err
    }

    info, err := f.Stat()
    if err != nil {
        return nil, err
    }
    ret, err := q.put(filename, f, info.Size())
    return ret, err
}

func (q *QClient) Put(path string, data []byte) (*Stat, error) {
    stat, err := q.put(path, bytes.NewReader(data), int64(len(data)))
    return stat, err
}

func (q *QClient) reader(path string, offset int64) (io.ReadCloser, error) {
    return nil, nil

}
