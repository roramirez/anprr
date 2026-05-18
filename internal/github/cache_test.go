package github

import (
	"sync"
	"testing"
	"time"
)

func TestCache_missOnEmpty(t *testing.T) {
	c := NewCache()
	_, ok := c.Get("key")
	if ok {
		t.Error("expected miss on empty cache")
	}
}

func TestCache_hitWithinTTL(t *testing.T) {
	c := NewCache()
	c.Set("k", "value", time.Minute)
	v, ok := c.Get("k")
	if !ok {
		t.Fatal("expected hit")
	}
	if v.(string) != "value" {
		t.Errorf("got %q", v)
	}
}

func TestCache_missAfterExpiry(t *testing.T) {
	c := NewCache()
	c.Set("k", "value", time.Millisecond)
	time.Sleep(5 * time.Millisecond)
	_, ok := c.Get("k")
	if ok {
		t.Error("expected miss after TTL expiry")
	}
}

func TestCache_invalidate(t *testing.T) {
	c := NewCache()
	c.Set("a", 1, time.Minute)
	c.Set("b", 2, time.Minute)
	c.Invalidate()
	if _, ok := c.Get("a"); ok {
		t.Error("expected miss after invalidate")
	}
}

func TestCache_race(t *testing.T) {
	c := NewCache()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			c.Set("k", i, time.Minute)
		}(i)
		go func() {
			defer wg.Done()
			c.Get("k")
		}()
	}
	wg.Wait()
}
