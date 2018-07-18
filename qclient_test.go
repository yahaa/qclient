package qclient

import (
    "testing"
    "io/ioutil"
)

const (
    ak  = "ak"
    sk  = "Q"
    bkt = "register"
    dm  = "data.foryung.com"
)


func TestQClient_PutFile(t *testing.T) {

    client := NewQClient(ak, sk, bkt, dm, false, true)
    st, err := client.PutFile("/Users/zihua/Desktop/register/test/de.gif")
    if err != nil {
        t.Error(err)
    } else {
        t.Log(st)
    }
}

func TestQClient_Put(t *testing.T) {

    path := "/Users/zihua/Downloads/aa.mp4"
    b, _ := ioutil.ReadFile(path)
    client := NewQClient(ak, sk, bkt, dm, false, true)
    st, err := client.Put(path, b)
    if err != nil {
        t.Error(err)
    } else {
        t.Log(st)
    }

}
