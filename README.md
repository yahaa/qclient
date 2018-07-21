# qclient
七牛对象存储接口封装,支持 linux/unix 系统，不支持 Windows 系统。

## 特性
* 对七牛 kodo sdk 进一步封装，简洁易用不啰嗦
* 支持给出了七牛云存储没有目录的解决方案
* 支持上传文件夹
* 支持分片上传
* 支持下载到指定目录
* 支持删除指定文件，指定目录下的所有文件

## 安装

```bash
$ go get -u github.com/yahaa/qclient
```

## 使用

```go
package main

import "github.com/yahaa/qclient"

func main() {
    ak := "your_ak"
    sk := "your_sk"
    bkt := "your_bucket"
    dm := "http://your_domain"
    client := qclient.NewQClient(ak, sk, bkt, dm, false, false)
    client.PushFile("your_file_name")
    client.Delete("your_file_paths")
    client.PushR("your_path")
    

}

```
