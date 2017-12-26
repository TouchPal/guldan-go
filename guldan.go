package guldan

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/orcaman/concurrent-map"
)

const (
	GULDAN_CLIENT_VERSION                    = "0.1.5"
	GULDAN_CLIENT_ITEM_EXPIRE_INTERVAL int32 = 5
	GULDAN_DEFAULT_ADDRESS                   = "http://localhost:7888"
)

type GuldanError string

func (e GuldanError) Error() string { return string(e) }

func debug(format string, args ...interface{}) {
	fmt.Printf("[DEBUG] "+format+"\n", args)
}

const (
	ErrGuldanNotFound        = GuldanError("not found")
	ErrGuldanForbidden       = GuldanError("forbidden")
	ErrGuldanBadConfigFormat = GuldanError("bad config format")
)

type CheckCallback func(string) bool
type NotifyCallback func(error, string, string)
type PrintCallback func(string)

type Item struct {
	ID      string
	Group   string
	Project string
	Name    string
	Value   string
	Token   string
	Version string
	Gray    bool
	Checker CheckCallback
	Notify  NotifyCallback
}

func newItem(gid, token string) (*Item, error) {
	s := strings.Split(gid, ".")
	if len(s) != 3 {
		return nil, errors.New("invalid gid{a.b.c}")
	}

	return &Item{
		ID:      gid,
		Group:   s[0],
		Project: s[1],
		Name:    s[2],
		Value:   "",
		Token:   token,
		Version: "",
		Gray:    false,
		Checker: nil,
	}, nil
}

type MissItem struct {
	Err     error
	Expired time.Time
}

func NewMissItem(err error, interval int32) *MissItem {
	return &MissItem{Err: err, Expired: time.Now().Add(time.Duration(interval) * time.Second)}
}

type GuldanClient struct {
	address          atomic.Value
	miss_cached_time int32
	expire_interval  int32
	items            cmap.ConcurrentMap
	watched          cmap.ConcurrentMap
	missed           cmap.ConcurrentMap
	printer          atomic.Value
	role             atomic.Value
}

func NewGuldanClient() *GuldanClient {
	c := &GuldanClient{
		miss_cached_time: 0,
		expire_interval:  GULDAN_CLIENT_ITEM_EXPIRE_INTERVAL,
		items:            cmap.New(),
		watched:          cmap.New(),
		missed:           cmap.New(),
	}
	c.address.Store(GULDAN_DEFAULT_ADDRESS)
	c.role.Store("client")
	return c
}

var initialized uint32
var instance *GuldanClient
var mu sync.Mutex

func GetInstance() *GuldanClient {
	if atomic.LoadUint32(&initialized) == 1 {
		return instance
	}
	mu.Lock()
	defer mu.Unlock()
	if initialized == 0 {
		instance = NewGuldanClient()
		atomic.StoreUint32(&initialized, 1)
	}
	return instance
}

func httpGet(url, token string) (*http.Response, error) {
	c := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	if len(token) > 0 {
		req.Header.Add("X-Guldan-Token", token)
	}
	return c.Do(req)
}

func (c *GuldanClient) pull(item *Item) (*Item, error) {
	var query_url string
	if item.Gray {
		query_url = fmt.Sprintf("%v/api/puller/%v/%v/%v?grey=true&cver=go%v&cid=%d&ctype=%v&lver=%v",
			c.address.Load().(string),
			item.Group,
			item.Project,
			item.Name,
			GULDAN_CLIENT_VERSION,
			os.Getpid(),
			c.role.Load().(string),
			item.Version)
	} else {
		query_url = fmt.Sprintf("%v/api/puller/%v/%v/%v?grey=false&cver=go%v&cid=%d&ctype=%v&lver=%v",
			c.address.Load().(string),
			item.Group,
			item.Project,
			item.Name,
			GULDAN_CLIENT_VERSION,
			os.Getpid(),
			c.role.Load().(string),
			item.Version)
	}

	r, err := httpGet(query_url, item.Token)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()

	if r.StatusCode == 404 {
		return nil, ErrGuldanNotFound
	} else if r.StatusCode == 403 {
		return nil, ErrGuldanForbidden
	} else if r.StatusCode != 200 {
		return nil, errors.New(fmt.Sprintf("error status code %d", r.StatusCode))
	}

	version := r.Header.Get("X-Guldan-Version")
	if len(version) == 0 {
		return nil, errors.New(fmt.Sprintf("response lost X-Guldan-Version"))
	}

	if strings.Compare(item.Version, version) == 0 {
		return item, nil
	}

	bytes, err1 := ioutil.ReadAll(r.Body)
	if err1 != nil {
		return nil, err1
	}
	body := string(bytes[:])

	if item.Checker == nil || (item.Checker != nil && item.Checker(body)) {
		return &Item{
			ID:      item.ID,
			Group:   item.Group,
			Project: item.Project,
			Name:    item.Name,
			Value:   body,
			Token:   item.Token,
			Version: version,
			Gray:    item.Gray,
			Checker: item.Checker,
			Notify:  item.Notify,
		}, nil
	}

	return item, nil
}

func getGGID(gid, token string, gray bool) string {
	if len(token) > 0 {
		gid = gid + ":" + token
	}
	if gray {
		return gid + ":gray"
	}
	return gid
}

func (c *GuldanClient) update(item *Item) {
	last_exists_item := item
	ggid := getGGID(item.ID, item.Token, item.Gray)
	for {
		time.Sleep(time.Second * time.Duration(atomic.LoadInt32(&c.expire_interval)))
		if t, err := c.pull(last_exists_item); err == nil {
			if t != last_exists_item {
				c.items.Set(ggid, t)
				if atomic.LoadInt32(&c.miss_cached_time) > 0 {
					c.missed.Remove(ggid)
				}
				if last_exists_item.Notify != nil {
					last_exists_item.Notify(nil, ggid, t.Value)
				}
				last_exists_item = t
			}
		} else if err == ErrGuldanNotFound || err == ErrGuldanForbidden {
			if c.items.Has(ggid) {
				c.items.Remove(ggid)
				if last_exists_item.Notify != nil {
					last_exists_item.Notify(err, ggid, "")
				}
			}
		} else {
			pointer := c.printer.Load()
			if pointer != nil {
				printer := pointer.(PrintCallback)
				printer(fmt.Sprintf("update %v ocurr %v", ggid, err.Error()))
			}
		}
	}
}

func (c *GuldanClient) Watch(gid, token string, gray bool, notify NotifyCallback, checker CheckCallback) error {
	ggid := getGGID(gid, token, gray)

	if c.watched.SetIfAbsent(ggid, true) == false {
		return nil
	}

	var item *Item
	if t, ok := c.items.Get(ggid); ok {
		item = t.(*Item)
	} else {
		var err error
		if item, err = c.RawGet(gid, token, true, gray); err != nil {
			c.watched.Remove(ggid)
			return err
		}
	}

	item.Checker = checker
	item.Notify = notify
	go c.update(item)

	return nil
}

func (c *GuldanClient) WatchPublic(gid string, gray bool, notify NotifyCallback, checker CheckCallback) error {
	return c.Watch(gid, "", gray, notify, checker)
}

func (c *GuldanClient) SetAddress(address string) {
	c.address.Store(address)
}

func (c *GuldanClient) SetItemExpireInterval(interval int32) {
	atomic.StoreInt32(&c.expire_interval, interval)
}

func (c *GuldanClient) SetRole(role string) {
	c.role.Store(role)
}

func (c *GuldanClient) SetPrinter(printer PrintCallback) {
	c.printer.Store(printer)
}

func (c *GuldanClient) SetMissCache(interval int32) {
	atomic.StoreInt32(&c.miss_cached_time, interval)
}

func (c *GuldanClient) RawGet(gid, token string, cached bool, gray bool) (*Item, error) {
	ggid := getGGID(gid, token, gray)

	if cached {
		if t, ok := c.items.Get(ggid); ok {
			return t.(*Item), nil
		}
		if atomic.LoadInt32(&c.miss_cached_time) > 0 {
			if t, ok := c.missed.Get(ggid); ok {
				miss := t.(*MissItem)
				if miss.Expired.Unix() > time.Now().Unix() {
					return nil, miss.Err
				}
				c.missed.Remove(ggid)
			}
		}
	}
	old_item, err := newItem(gid, token)
	if err != nil {
		return nil, err
	}
	old_item.Gray = gray

	// in Get method never happen old_item.Version == new_item.Version
	new_item, err := c.pull(old_item)
	if err != nil {
		if atomic.LoadInt32(&c.miss_cached_time) > 0 {
			if err == ErrGuldanNotFound || err == ErrGuldanForbidden {
				c.missed.Set(ggid, NewMissItem(err, c.miss_cached_time))
			}
		}
		return nil, err
	}
	if new_item == old_item {
		return nil, ErrGuldanBadConfigFormat
	}

	if cached {
		c.items.Set(ggid, new_item)
		if atomic.LoadInt32(&c.miss_cached_time) > 0 {
			c.missed.Remove(ggid)
		}
	}
	return new_item, nil
}

func (c *GuldanClient) Get(gid, token string, cached bool, gray bool) (string, error) {
	new_item, err := c.RawGet(gid, token, cached, gray)
	if err != nil {
		return "", err
	}
	return new_item.Value, nil
}

func (c *GuldanClient) GetPublic(gid string, cached bool, gray bool) (string, error) {
	return c.Get(gid, "", cached, gray)
}

func (c *GuldanClient) CachedCount() int {
	return c.items.Count()
}
