package guldan

import (
	"fmt"
	"testing"
	"time"
)

type Object struct {
	ID    string
	Value string
}

func NewObject() *Object {
	return &Object{ID: "123", Value: "456"}
}

func Test_GolangPointer(t *testing.T) {
	m := make(map[string]interface{})
	item := NewObject()
	m["hello"] = item
	delete(m, "hello")
	t.Logf("ID: %v, Value: %v", item.ID, item.Value)
}

func Test_GetPublic(t *testing.T) {
	g := GetInstance()
	body, err := g.GetPublic("public.hello.world", false, false)
	if err != nil {
		t.Fatal(err.Error())
		t.Fail()
	}
	t.Logf("config = %v", body)
}

func Test_GetPrivateErrorToken(t *testing.T) {
	g := GetInstance()
	{
		_, err := g.Get("helloworld.proj.test", "", false, false)
		if err != ErrGuldanForbidden {
			t.Fatal(err.Error())
			t.Fail()
		}
	}
	{
		_, err := g.Get("helloworld.proj.test", "", true, false)
		if err != ErrGuldanForbidden {
			t.Fatal(err.Error())
			t.Fail()
		}
	}
}

func Test_GetPrivateCorrectToken(t *testing.T) {
	g := GetInstance()
	{
		_, err := g.Get("helloworld.proj.test", "ced4a335c533c8328dec4704a2513fe8", false, false)
		if err != nil {
			t.Fatal(err.Error())
			t.Fail()
		}
	}
	{
		_, err := g.Get("helloworld.proj.test", "", true, false)
		if err != ErrGuldanForbidden {
			t.Fatal(err.Error())
			t.Fail()
		}
	}
	{
		_, err := g.Get("helloworld.proj.test", "ced4a335c533c8328dec4704a2513fe8", true, false)
		if err != nil {
			t.Fatal(err.Error())
			t.Fail()
		}
	}
	{
		_, err := g.Get("helloworld.proj.test", "", true, false)
		if err != ErrGuldanForbidden {
			t.Fatal(err.Error())
			t.Fail()
		}
	}
	{
		body, err := g.Get("helloworld.proj.test", "ced4a335c533c8328dec4704a2513fe8", true, false)
		if err != nil {
			t.Fatal(err.Error())
			t.Fail()
		}
		t.Logf("config = %v", body)
	}
}

func Test_EnableMissCache(t *testing.T) {
	g := GetInstance()
	g.SetMissCache(10)
	for i := 0; i < 100; i++ {
		g.GetPublic("public.hello.xx", true, false)
	}
}

func Test_WatchPublicNotNotify(t *testing.T) {
	if err := GetInstance().WatchPublic("public.hello.world", false, nil, nil); err != nil {
		t.Fatal(err.Error())
	}
	time.Sleep(time.Duration(20) * time.Second)
}

func Test_EnableMissCache2(t *testing.T) {
	g := GetInstance()
	g.SetMissCache(1)
	for j := 0; j < 10; j++ {
		for i := 0; i < 10; i++ {
			g.GetPublic("public.hello.yy", true, false)
		}
		time.Sleep(time.Duration(2) * time.Second)
	}
}

func getter(gid string) {
	g := GetInstance()
	i := 0
	for i < 60 {
		time.Sleep(time.Second)
		body, err := g.GetPublic("public.hello.world", true, false)
		if err != nil {
			fmt.Println(err.Error())
		}
		fmt.Printf("getter config = %v\n", body)
		i = i + 1
	}
}

func Test_WatchPublic(t *testing.T) {
	g := GetInstance()
	fn := func(err error, ggid, body string) {
		if err != nil {
			t.Logf("occur error %v", err.Error())
		} else {
			t.Logf("now config %v = %v", ggid, body)
		}
	}
	if err := g.WatchPublic("public.hello.world", false, fn, nil); err != nil {
		t.Fatal(err.Error())
	}

	go getter("public.hello.world")

	time.Sleep(time.Duration(60) * time.Second)
}
