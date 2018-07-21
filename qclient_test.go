package qclient

import (
    "testing"
    "io/ioutil"
    "fmt"
    "path"
    "strings"
    "github.com/qiniu/api.v7/storage"
)

const (
    ak  = "your_ak"
    sk  = "your_sk"
    bkt = "your_bucket"
    dm  = "http://your_domain"
)

func TestQClient_PushFile(t *testing.T) {

    client := NewQClient(ak, sk, bkt, dm, false, false)
    st, err := client.PushFile("README.md")
    if err != nil {
        t.Error(err)
    } else {
        t.Log(st)
    }
}

func rprint(dir string) {
    fs, err := ioutil.ReadDir(dir)
    if err != nil {
        return
    }

    for _, f := range fs {
        if f.IsDir() {
            rprint(dir + "/" + f.Name())
        } else {
            fmt.Println(dir + "/" + f.Name())
        }
    }

}

func TestQClient_PushR(t *testing.T) {
    rprint("../qclient")
}

func TestQClient_Push(t *testing.T) {

    path := "/Users/zihua/Downloads/aa.mp4"
    b, _ := ioutil.ReadFile(path)
    client := NewQClient(ak, sk, bkt, dm, false, false)

    st, err := client.Push(path, b)
    if err != nil {
        t.Error(err)
    } else {
        t.Log(st)
    }
}

func TestQClient_GetURI(t *testing.T) {
    client := NewQClient(ak, sk, bkt, dm, false, false)
    path := "jp.gif"
    res := client.URLFor(path)
    t.Log(res)
}

func TestQClient_GetAndSave(t *testing.T) {
    client := NewQClient(ak, sk, bkt, dm, false, false)
    path := "/README.md"
    savePath := "/Users/zihua/Desktop/register/666"
    err := client.PullTo(path, savePath)
    if err != nil {
        t.Error(err)
    } else {
        t.Log("success")
    }
}

func TestQClient_PushR2(t *testing.T) {
    client := NewQClient(ak, sk, bkt, dm, false, true)

    path := "/Users/zihua/Desktop"

    sts := client.PushR(path)
    fmt.Println(sts)
}

func TestQClient_PushR3(t *testing.T) {
    fullFilename := ""
    fmt.Println("fullFilename =", fullFilename)
    var filenameWithSuffix string
    filenameWithSuffix = path.Base(fullFilename) //获取文件名带后缀
    fmt.Println("filenameWithSuffix =", filenameWithSuffix)
    var fileSuffix string
    fileSuffix = path.Ext(filenameWithSuffix) //获取文件后缀
    fmt.Println("fileSuffix =", fileSuffix)

    var filenameOnly string
    filenameOnly = strings.TrimSuffix(filenameWithSuffix, fileSuffix) //获取文件名
    fmt.Println("filenameOnly =", filenameOnly)
}

func TestQClient_List(t *testing.T) {
    client := NewQClient(ak, sk, bkt, dm, false, true)
    fis := client.List("/Users/zihua/Desktop/register")

    for _, v := range fis {
        fmt.Println(v.Name, v.IsDir)
        fmt.Printf("%#v", v)
    }

}

func TestNewQClient(t *testing.T) {
    client := NewQClient(ak, sk, bkt, dm, false, true)
    item := storage.ListItem{
        Key: "/Users/zihua/Downloads/go 系列/L001-Go语言-mp4/01 Go开发1期 day1 开课介绍01.mp4",
    }

    path := "/Users/zihua/Downloads"
    fis, ok := client.trimPath(path, item)
    if ok {
        t.Log(fis.Name)
    } else {
        t.Error(ok)
    }

}

func TestQClient_Delete(t *testing.T) {
    client := NewQClient(ak, sk, bkt, dm, false, true)
    st := client.Delete("/Users/zihua/Desktop/register/")
    for _, v := range st {
        fmt.Println(v.Status, v.Message)
    }
}
