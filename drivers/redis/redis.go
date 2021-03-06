// Copyright (c) 2018 Dmytro Lahoza <dmitry@lagoza.name>
//
// Permission is hereby granted, free of charge, to any person obtaining
// a copy of this software and associated documentation files (the
// "Software"), to deal in the Software without restriction, including
// without limitation the rights to use, copy, modify, merge, publish,
// distribute, sublicense, and/or sell copies of the Software, and to
// permit persons to whom the Software is furnished to do so, subject to
// the following conditions:
//
// The above copyright notice and this permission notice shall be
// included in all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
// EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
// MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
// NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE
// LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION
// OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION
// WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

package redis

import (
	"time"

	"github.com/DLag/cachery"
	"github.com/garyburd/redigo/redis"
)

// Driver type satisfies cachery.Driver interface
type Driver struct {
	client *redis.Pool
}

// New creates redis driver instance
func New(redis *redis.Pool) *Driver {
	driver := new(Driver)
	driver.client = redis
	return driver
}

// DefaultPool creates redis pool with single host
func DefaultPool(host string, maxIdle int, idleTimeout time.Duration) *redis.Pool {
	return &redis.Pool{
		MaxIdle:     maxIdle,
		IdleTimeout: idleTimeout,
		Dial:        func() (redis.Conn, error) { return redis.Dial("tcp", host) },
	}
}

// Invalidate removes the key from the cache store
func (c *Driver) Invalidate(cacheName string, key interface{}) error {
	return c.del(cacheName, cachery.Key(key))
}

// InvalidateAll removes all keys from the cache store
func (c *Driver) InvalidateAll(cacheName string) {
	_ = c.delSet(cacheName)
}

// Set saves key to the cache store
func (c *Driver) Set(cacheName string, key interface{}, val []byte, ttl time.Duration) (err error) {
	skey := cachery.Key(key)
	client := c.client.Get()
	defer func() {
		e := client.Close()
		if err == nil {
			err = e
		}
	}()
	if err = client.Send("SADD", cacheName, cacheName+":"+skey); err != nil {
		return
	}
	if err = client.Send("SET", cacheName+":"+skey, val); err != nil {
		return
	}
	if err = client.Send("EXPIRE", cacheName+":"+skey, ttl.Seconds()); err != nil {
		return
	}
	if err = client.Flush(); err != nil {
		return
	}
	return
}

// Get loads key from the cache store if it is not outdated
func (c *Driver) Get(cacheName string, key interface{}) (val []byte, ttl time.Duration, err error) {
	skey := cachery.Key(key)
	client := c.client.Get()
	defer func() {
		e := client.Close()
		if err == nil {
			err = e
		}
	}()
	val, err = redis.Bytes(client.Do("GET", cacheName+":"+skey))
	if err != nil {
		return
	}
	var rawttl int
	rawttl, err = redis.Int(client.Do("TTL", cacheName+":"+skey))
	ttl = time.Second * time.Duration(rawttl)
	return
}

func (c *Driver) delSet(cacheName string) (err error) {
	client := c.client.Get()
	defer func() {
		e := client.Close()
		if err == nil {
			err = e
		}
	}()
	members, err := redis.Strings(client.Do("SMEMBERS", cacheName))
	if err != nil {
		return err
	}
	for _, m := range members {
		_ = client.Send("SREM", cacheName, m)
		_ = client.Send("DEL", m)
	}
	err = client.Flush()
	return
}

func (c *Driver) del(cacheName string, key string) (err error) {
	client := c.client.Get()
	defer func() {
		e := client.Close()
		if err == nil {
			err = e
		}
	}()
	_ = client.Send("SREM", cacheName, cacheName+":"+key)
	_ = client.Send("DEL", cacheName+":"+key)
	err = client.Flush()
	return
}
