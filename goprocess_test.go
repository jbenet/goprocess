package goprocess

import (
	"testing"
	"time"
)

type tree struct {
	Process
	c []tree
}

func setupHierarchy(p Process) tree {
	t := func(n Process, ts ...tree) tree {
		return tree{n, ts}
	}

	a := WithParent(p)
	b1 := WithParent(a)
	b2 := WithParent(a)
	c1 := WithParent(b1)
	c2 := WithParent(b1)
	c3 := WithParent(b2)
	c4 := WithParent(b2)

	return t(a, t(b1, t(c1), t(c2)), t(b2, t(c3), t(c4)))
}

func TestClosingClosed(t *testing.T) {

	a := WithParent(Background())
	b := WithParent(a)

	Q := make(chan string, 3)

	go func() {
		<-a.Closing()
		Q <- "closing"
		b.Close()
	}()

	go func() {
		<-a.Closed()
		Q <- "closed"
	}()

	go func() {
		a.Close()
		Q <- "closed"
	}()

	if q := <-Q; q != "closing" {
		t.Error("order incorrect. closing not first")
	}
	if q := <-Q; q != "closed" {
		t.Error("order incorrect. closing not first")
	}
	if q := <-Q; q != "closed" {
		t.Error("order incorrect. closing not first")
	}
}

func TestChildFunc(t *testing.T) {
	a := WithParent(Background())

	wait1 := make(chan struct{})
	wait2 := make(chan struct{})
	wait3 := make(chan struct{})
	wait4 := make(chan struct{})
	go func() {
		a.Close()
		wait4 <- struct{}{}
	}()

	a.Go(func(process Process) {
		wait1 <- struct{}{}
		<-wait2
		wait3 <- struct{}{}
	})

	<-wait1
	select {
	case <-wait3:
		t.Error("should not be closed yet")
	case <-wait4:
		t.Error("should not be closed yet")
	case <-a.Closed():
		t.Error("should not be closed yet")
	default:
	}

	wait2 <- struct{}{}

	select {
	case <-wait3:
	case <-time.After(time.Second):
		t.Error("should be closed now")
	}

	select {
	case <-wait4:
	case <-time.After(time.Second):
		t.Error("should be closed now")
	}
}

func TestTeardownCalledOnce(t *testing.T) {
	a := setupHierarchy(Background())

	onlyOnce := func() func() error {
		count := 0
		return func() error {
			count++
			if count > 1 {
				t.Error("called", count, "times")
			}
			return nil
		}
	}

	setTeardown := func(t tree, tf TeardownFunc) {
		t.Process.(*process).teardown = tf
	}

	setTeardown(a, onlyOnce())
	setTeardown(a.c[0], onlyOnce())
	setTeardown(a.c[0].c[0], onlyOnce())
	setTeardown(a.c[0].c[1], onlyOnce())
	setTeardown(a.c[1], onlyOnce())
	setTeardown(a.c[1].c[0], onlyOnce())
	setTeardown(a.c[1].c[1], onlyOnce())

	a.c[0].c[0].Close()
	a.c[0].c[0].Close()
	a.c[0].c[0].Close()
	a.c[0].c[0].Close()
	a.c[0].Close()
	a.c[0].Close()
	a.c[0].Close()
	a.c[0].Close()
	a.Close()
	a.Close()
	a.Close()
	a.Close()
	a.c[1].Close()
	a.c[1].Close()
	a.c[1].Close()
	a.c[1].Close()
}

func TestOnClosed(t *testing.T) {

	p := WithParent(Background())
	a := setupHierarchy(p)
	Q := make(chan string, 10)

	onClosed := func(s string, p Process) {
		<-p.Closed()
		Q <- s
	}

	go onClosed("0", a.c[0])
	go onClosed("10", a.c[1].c[0])
	go onClosed("", a)
	go onClosed("00", a.c[0].c[0])
	go onClosed("1", a.c[1])
	go onClosed("01", a.c[0].c[1])
	go onClosed("11", a.c[1].c[1])

	test := func(ss ...string) {
		s1 := <-Q
		for _, s2 := range ss {
			if s1 == s2 {
				return
			}
		}
		t.Error("context not in group", s1, ss)
	}

	go p.Close()

	test("00", "01", "10", "11")
	test("00", "01", "10", "11")
	test("00", "01", "10", "11")
	test("00", "01", "10", "11")
	test("0", "1")
	test("0", "1")
	test("")
}
