package fsnotify

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify/internal"
)

// Set soft open file limit to the maximum; on e.g. OpenBSD it's 512/1024.
//
// Go 1.19 will always do this when the os package is imported.
//
// https://go-review.googlesource.com/c/go/+/393354/
func init() {
	internal.SetRlimit()
	enableRecurse = true
}

func TestScript(t *testing.T) {
	err := filepath.Walk("./testdata", func(path string, info fs.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		n := strings.Split(filepath.ToSlash(path), "/")
		t.Run(strings.Join(n[1:], "/"), func(t *testing.T) {
			t.Parallel()
			d, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			parseScript(t, string(d))
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// Multiple writes to a file with the same fd.
func TestWatchMultipleWrite(t *testing.T) {
	t.Parallel()
	w := newCollector(t)
	w.collect(t)
	tmp := t.TempDir()

	echoAppend(t, "data", tmp, "file")
	addWatch(t, w.w, tmp)
	fp, err := os.OpenFile(join(tmp, "file"), os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fp.Write([]byte("X")); err != nil {
		t.Fatal(err)
	}
	if err := fp.Sync(); err != nil {
		t.Fatal(err)
	}
	eventSeparator()
	if _, err := fp.Write([]byte("Y")); err != nil {
		t.Fatal(err)
	}
	if err := fp.Close(); err != nil {
		t.Fatal(err)
	}

	cmpEvents(t, tmp, w.stop(t), newEvents(t, `
		write  /file  # write X
		write  /file  # write Y
	`))
}

// Remove watched file with open fd
func TestWatchRemoveOpenFd(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows hard-locks open files so this will never work")
	}

	t.Parallel()
	tmp := t.TempDir()
	w := newCollector(t)
	w.collect(t)

	touch(t, tmp, "/file")

	fp, err := os.Open(join(tmp, "/file"))
	if err != nil {
		t.Fatal(err)
	}
	defer fp.Close()

	addWatch(t, w.w, tmp, "/file")
	rm(t, tmp, "/file")

	cmpEvents(t, tmp, w.stop(t), newEvents(t, `
		remove   /file

		# inotify will just emit a CHMOD for the unlink, but won't actually
		# emit a REMOVE until the descriptor is closed. Bit odd, but not much
		# we can do about it. The REMOVE is tested in TestInotifyDeleteOpenFile()
		linux:
			chmod  /file
	`))
}

// Remove watched directory
func TestWatchRemoveWatchedDir(t *testing.T) {
	if runtime.GOOS == "dragonfly" {
		t.Skip("broken: inconsistent events") // TODO
	}

	t.Parallel()
	tmp := t.TempDir()
	w := newCollector(t)
	w.collect(t)

	touch(t, tmp, "a")
	touch(t, tmp, "b")
	touch(t, tmp, "c")
	touch(t, tmp, "d")
	touch(t, tmp, "e")
	touch(t, tmp, "f")
	touch(t, tmp, "g")
	mkdir(t, tmp, "h")
	mkdir(t, tmp, "h", "a")
	mkdir(t, tmp, "i")
	mkdir(t, tmp, "i", "a")
	mkdir(t, tmp, "j")
	mkdir(t, tmp, "j", "a")
	addWatch(t, w.w, tmp)
	rmAll(t, tmp)

	if runtime.GOOS != "windows" {
		cmpEvents(t, tmp, w.stop(t), newEvents(t, `
				remove    /
				remove    /a
				remove    /b
				remove    /c
				remove    /d
				remove    /e
				remove    /f
				remove    /g
				remove    /h
				remove    /i
				remove    /j`))
		return
	}

	// ReadDirectoryChangesW gives undefined results: not all files are
	// always present. So test only that 1) we got the directory itself, and
	// 2) we don't get events for unspected files.
	var (
		events = w.stop(t)
		found  bool
	)
	for _, e := range events {
		if e.Name == tmp && e.Has(Remove) {
			found = true
			continue
		}
		if filepath.Dir(e.Name) != tmp {
			t.Errorf("unexpected event: %s", e)
		}
	}
	if !found {
		t.Fatalf("didn't see directory in:\n%s", events)
	}
}

func TestClose(t *testing.T) {
	chanClosed := func(t *testing.T, w *Watcher) {
		t.Helper()

		// Need a small sleep as Close() on kqueue does all sorts of things,
		// which may take a little bit.
		switch runtime.GOOS {
		case "freebsd", "openbsd", "netbsd", "dragonfly", "darwin", "solaris", "illumos":
			time.Sleep(50 * time.Millisecond)
		}

		tim := time.NewTimer(50 * time.Millisecond)
	loop:
		for {
			select {
			default:
				t.Fatal("blocking on Events")
			case <-tim.C:
				t.Fatalf("Events not closed")
			case _, ok := <-w.Events:
				if !ok {
					break loop
				}
			}
		}

		select {
		default:
			t.Fatal("blocking on Errors")
		case err, ok := <-w.Errors:
			if ok {
				t.Fatalf("Errors not closed; read:\n\t%s", err)
			}
		}
	}

	t.Run("close", func(t *testing.T) {
		t.Parallel()

		w := newWatcher(t)
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		chanClosed(t, w)

		var done int32
		go func() {
			w.Close()
			atomic.StoreInt32(&done, 1)
		}()

		eventSeparator()
		if atomic.LoadInt32(&done) == 0 {
			t.Fatal("double Close() test failed: second Close() call didn't return")
		}

		if err := w.Add(t.TempDir()); err == nil {
			t.Fatal("expected error on Watch() after Close(), got nil")
		}
	})

	// Make sure that Close() works even when the Events channel isn't being
	// read.
	t.Run("events not read", func(t *testing.T) {
		t.Parallel()

		tmp := t.TempDir()
		w := newWatcher(t, tmp)

		touch(t, tmp, "file")
		rm(t, tmp, "file")
		eventSeparator()
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}

		// TODO: windows backend doesn't work well here; can't easily fix it.
		//       Need to rewrite things a bit.
		if runtime.GOOS != "windows" {
			chanClosed(t, w)
		}
	})

	// Make sure that calling Close() while REMOVE events are emitted doesn't race.
	t.Run("close while removing files", func(t *testing.T) {
		t.Parallel()
		tmp := t.TempDir()

		files := make([]string, 0, 200)
		for i := 0; i < 200; i++ {
			f := join(tmp, fmt.Sprintf("file-%03d", i))
			touch(t, f, noWait)
			files = append(files, f)
		}

		w := newWatcher(t, tmp)

		startC, stopC, errC := make(chan struct{}), make(chan struct{}), make(chan error)
		go func() {
			for {
				select {
				case <-w.Errors:
				case <-w.Events:
				case <-stopC:
					return
				}
			}
		}()
		rmDone := make(chan struct{})
		go func() {
			<-startC
			for _, f := range files {
				rm(t, f, noWait)
			}
			rmDone <- struct{}{}
		}()
		go func() {
			<-startC
			errC <- w.Close()
		}()
		close(startC)
		defer close(stopC)
		if err := <-errC; err != nil {
			t.Fatal(err)
		}

		<-rmDone
	})

	// Make sure Close() doesn't race when called more than once; hard to write
	// a good reproducible test for this, but running it 150 times seems to
	// reproduce it in ~75% of cases and isn't too slow (~0.06s on my system).
	t.Run("double close", func(t *testing.T) {
		t.Run("default", func(t *testing.T) {
			t.Parallel()

			for i := 0; i < 150; i++ {
				w, err := NewWatcher()
				if err != nil {
					if strings.Contains(err.Error(), "too many") { // syscall.EMFILE
						time.Sleep(100 * time.Millisecond)
						continue
					}
					t.Fatal(err)
				}
				go w.Close()
				go w.Close()
				go w.Close()
			}
		})
		t.Run("buffered=4096", func(t *testing.T) {
			t.Parallel()

			for i := 0; i < 150; i++ {
				w, err := NewBufferedWatcher(4096)
				if err != nil {
					if strings.Contains(err.Error(), "too many") { // syscall.EMFILE
						time.Sleep(100 * time.Millisecond)
						continue
					}
					t.Fatal(err)
				}
				go w.Close()
				go w.Close()
				go w.Close()
			}
		})
	})

	t.Run("closes channels after read", func(t *testing.T) {
		if runtime.GOOS == "netbsd" {
			t.Skip("flaky") // TODO
		}

		t.Parallel()

		tmp := t.TempDir()

		w := newCollector(t, tmp)
		w.collect(t)
		touch(t, tmp, "qwe")
		touch(t, tmp, "asd")

		if err := w.w.Close(); err != nil {
			t.Fatal(err)
		}

		chanClosed(t, w.w)
	})

	t.Run("error after closed", func(t *testing.T) {
		t.Parallel()

		tmp := t.TempDir()
		w := newWatcher(t, tmp)
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}

		file := join(tmp, "file")
		touch(t, file)
		if err := w.Add(file); !errors.Is(err, ErrClosed) {
			t.Fatalf("wrong error for Add: %#v", err)
		}
		if err := w.Remove(file); err != nil {
			t.Fatalf("wrong error for Remove: %#v", err)
		}
		if l := w.WatchList(); l != nil { // Should return an error, but meh :-/
			t.Fatalf("WatchList not nil: %#v", l)
		}
	})
}

func TestAdd(t *testing.T) {
	t.Run("doesn't exist", func(t *testing.T) {
		t.Parallel()
		tmp := t.TempDir()

		w := newWatcher(t)
		err := w.Add(join(tmp, "non-existent"))
		if err == nil {
			t.Fatal("err is nil")
		}

		// TODO(v2): errors for this are inconsistent; should be fixed in v2. See #144
		switch runtime.GOOS {
		case "linux":
			if _, ok := err.(syscall.Errno); !ok {
				t.Errorf("wrong error type: %[1]T: %#[1]v", err)
			}
		case "windows":
			if _, ok := err.(*os.SyscallError); !ok {
				t.Errorf("wrong error type: %[1]T: %#[1]v", err)
			}
		default:
			if _, ok := err.(*fs.PathError); !ok {
				t.Errorf("wrong error type: %[1]T: %#[1]v", err)
			}
		}
	})

	t.Run("permission denied", func(t *testing.T) {
		t.Parallel()
		if runtime.GOOS == "windows" {
			t.Skip("chmod doesn't work on Windows") // TODO: see if we can make a file unreadable
		}

		tmp := t.TempDir()
		dir := join(tmp, "dir-unreadable")
		mkdir(t, dir)
		touch(t, dir, "/file")
		chmod(t, 0, dir)

		w := newWatcher(t)
		defer func() {
			w.Close()
			chmod(t, 0o755, dir) // Make TempDir() cleanup work
		}()
		err := w.Add(dir)
		if err == nil {
			t.Fatal("error is nil")
		}
		if !errors.Is(err, internal.ErrUnixEACCES) {
			t.Errorf("not unix.EACCESS: %T %#[1]v", err)
		}
		if !errors.Is(err, internal.ErrSyscallEACCES) {
			t.Errorf("not syscall.EACCESS: %T %#[1]v", err)
		}
	})

	// The second Add() should be a no-op
	t.Run("add same dir twice", func(t *testing.T) {
		tmp := t.TempDir()
		w := newCollector(t)
		if err := w.w.Add(tmp); err != nil {
			t.Fatal(err)
		}
		if err := w.w.Add(tmp); err != nil {
			t.Fatal(err)
		}

		w.collect(t)
		touch(t, tmp, "file")
		rm(t, tmp, "file")

		cmpEvents(t, tmp, w.events(t), newEvents(t, `
			create /file
			remove /file
		`))
	})
	t.Run("add same dir twice through symlink", func(t *testing.T) {
		t.Parallel()
		if isSolaris() {
			t.Skip("broken: links are resolved and added twice") // TODO: should fix
		}
		if !internal.HasPrivilegesForSymlink() {
			t.Skip("admin permissions required on Windows")
		}

		tmp := t.TempDir()
		mkdir(t, tmp, "dir")
		symlink(t, join(tmp, "dir"), tmp, "link")
		w := newCollector(t)
		if err := w.w.Add(join(tmp, "dir")); err != nil {
			t.Fatal(err)
		}
		if err := w.w.Add(join(tmp, "link")); err != nil {
			t.Fatal(err)
		}

		w.collect(t)
		touch(t, tmp, "dir/file")
		rm(t, tmp, "dir/file")

		cmpEvents(t, tmp, w.events(t), newEvents(t, `
			create /dir/file
			remove /dir/file
		`))
	})

	t.Run("add same file twice", func(t *testing.T) {
		t.Parallel()
		tmp := t.TempDir()
		touch(t, tmp, "file")

		w := newCollector(t)
		if err := w.w.Add(join(tmp, "file")); err != nil {
			t.Fatal(err)
		}
		if err := w.w.Add(join(tmp, "file")); err != nil {
			t.Fatal(err)
		}

		w.collect(t)
		echoAppend(t, "aaa", tmp, "file")
		rm(t, tmp, "file")

		cmpEvents(t, tmp, w.events(t), newEvents(t, `
			write /file
			remove /file

			linux:
				write /file
				chmod /file
				remove /file
		`))
	})
	t.Run("add same file twice through symlink", func(t *testing.T) {
		t.Parallel()
		if isSolaris() {
			t.Skip("broken: links are resolved and added twice") // TODO: should fix
		}
		if !internal.HasPrivilegesForSymlink() {
			t.Skip("admin permissions required on Windows")
		}

		tmp := t.TempDir()
		touch(t, tmp, "file")
		symlink(t, join(tmp, "file"), tmp, "link")

		w := newCollector(t)
		if err := w.w.Add(join(tmp, "file")); err != nil {
			t.Fatal(err)
		}
		if err := w.w.Add(join(tmp, "link")); err != nil {
			t.Fatal(err)
		}

		w.collect(t)
		echoAppend(t, "aaa", tmp, "file")
		rm(t, tmp, "file")

		cmpEvents(t, tmp, w.events(t), newEvents(t, `
			write /file
			remove /file

			linux:
				write /file
				chmod /file
				remove /file
		`))
	})

	t.Run("not reading events", func(t *testing.T) {
		t.Parallel()

		w := newWatcher(t)
		defer w.Close()

		tmp := t.TempDir()
		mkdir(t, tmp, "/dir1")
		mkdir(t, tmp, "/dir2")
		addWatch(t, w, tmp, "/dir1")
		addWatch(t, w, tmp, "/dir2")

		{
			have, want := w.WatchList(), []string{join(tmp, "/dir1"), join(tmp, "/dir2")}
			sort.Strings(have)
			if !reflect.DeepEqual(have, want) {
				t.Errorf("\nhave: %s\nwant: %s", have, want)
			}
		}
		if err := w.Remove(join(tmp, "/dir1")); err != nil {
			t.Fatal(err)
		}
		{
			have, want := w.WatchList(), []string{join(tmp, "/dir2")}
			sort.Strings(have)
			if !reflect.DeepEqual(have, want) {
				t.Errorf("\nhave: %s\nwant: %s", have, want)
			}
		}
	})
}

func TestRemove(t *testing.T) {
	t.Run("works", func(t *testing.T) {
		t.Parallel()

		tmp := t.TempDir()
		touch(t, tmp, "file")

		w := newCollector(t)
		w.collect(t)
		addWatch(t, w.w, tmp)
		if err := w.w.Remove(tmp); err != nil {
			t.Fatal(err)
		}

		time.Sleep(200 * time.Millisecond)
		echoAppend(t, "data", tmp, "file")
		chmod(t, 0o700, tmp, "file")

		have := w.stop(t)
		if len(have) > 0 {
			t.Errorf("received events; expected none:\n%s", have)
		}
	})

	t.Run("remove same dir twice", func(t *testing.T) {
		t.Parallel()

		tmp := t.TempDir()

		touch(t, tmp, "file")

		w := newWatcher(t)
		defer w.Close()

		addWatch(t, w, tmp)

		if err := w.Remove(tmp); err != nil {
			t.Fatal(err)
		}
		err := w.Remove(tmp)
		if err == nil {
			t.Fatal("no error")
		}
		if !errors.Is(err, ErrNonExistentWatch) {
			t.Fatalf("wrong error: %T", err)
		}
	})

	t.Run("remove same dir twice through symlink", func(t *testing.T) {
		t.Parallel()
		if isSolaris() {
			t.Skip("broken: links are resolved and added twice") // TODO: should fix
		}
		if !internal.HasPrivilegesForSymlink() {
			t.Skip("admin permissions required on Windows")
		}

		tmp := t.TempDir()
		mkdir(t, tmp, "dir")
		symlink(t, join(tmp, "dir"), tmp, "link")

		w := newWatcher(t)
		defer w.Close()

		addWatch(t, w, tmp, "dir")
		addWatch(t, w, tmp, "link")

		if err := w.Remove(join(tmp, "dir")); err != nil {
			t.Fatal(err)
		}
		err := w.Remove(join(tmp, "link"))
		if err == nil {
			t.Fatal("no error")
		}
		if !errors.Is(err, ErrNonExistentWatch) {
			t.Fatalf("wrong error: %T", err)
		}
	})

	t.Run("remove same file twice", func(t *testing.T) {
		t.Parallel()

		tmp := t.TempDir()
		touch(t, tmp, "file")

		w := newWatcher(t)
		defer w.Close()

		addWatch(t, w, tmp, "file")

		if err := w.Remove(join(tmp, "file")); err != nil {
			t.Fatal(err)
		}
		err := w.Remove(join(tmp, "file"))
		if err == nil {
			t.Fatal("no error")
		}
		if !errors.Is(err, ErrNonExistentWatch) {
			t.Fatalf("wrong error: %T", err)
		}
	})

	t.Run("remove same file twice through symlink", func(t *testing.T) {
		t.Parallel()
		if isSolaris() {
			t.Skip("broken: links are resolved and added twice") // TODO: should fix
		}
		if !internal.HasPrivilegesForSymlink() {
			t.Skip("admin permissions required on Windows")
		}
		if runtime.GOOS == "windows" {
			// TODO: I'm not sure if this is due to a problem in our code, or
			// just a limitation of Windows, but very few people are using links
			// in Windows and even fewer links to files, so whatever.
			t.Skip("fails on Windows")
		}

		tmp := t.TempDir()
		touch(t, tmp, "file")
		symlink(t, join(tmp, "file"), tmp, "link")

		w := newWatcher(t)
		defer w.Close()

		addWatch(t, w, tmp, "file")
		addWatch(t, w, tmp, "link")

		if err := w.Remove(join(tmp, "file")); err != nil {
			t.Fatal(err)
		}
		err := w.Remove(join(tmp, "link"))
		if err == nil {
			t.Fatal("no error")
		}
		if !errors.Is(err, ErrNonExistentWatch) {
			t.Fatalf("wrong error: %T", err)
		}
	})

	// Make sure that concurrent calls to Remove() don't race.
	t.Run("no race", func(t *testing.T) {
		t.Parallel()

		tmp := t.TempDir()
		touch(t, tmp, "file")

		for i := 0; i < 10; i++ {
			w := newWatcher(t)
			defer w.Close()
			addWatch(t, w, tmp)

			done := make(chan struct{})
			go func() {
				defer func() { done <- struct{}{} }()
				w.Remove(tmp)
			}()
			go func() {
				defer func() { done <- struct{}{} }()
				w.Remove(tmp)
			}()
			<-done
			<-done
			w.Close()
		}
	})

	// Make sure file handles are correctly released.
	//
	// regression test for #42 see https://gist.github.com/timshannon/603f92824c5294269797
	t.Run("release file handles", func(t *testing.T) {
		w := newWatcher(t)
		defer w.Close()

		// consume the events
		var werr error
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case werr = <-w.Errors:
					return
				case <-w.Events:
				}
			}
		}()

		tmp := t.TempDir()
		dir := join(tmp, "child")
		addWatch(t, w, tmp)
		mkdir(t, dir)
		addWatch(t, w, dir) // start watching child
		rmWatch(t, w, dir)  // stop watching child
		rmAll(t, dir)       // delete child dir

		// Child dir should no longer exist
		_, err := os.Stat(dir)
		if err == nil {
			t.Fatalf("dir %q should no longer exist!", dir)
		}
		if _, ok := err.(*os.PathError); err != nil && !ok {
			t.Errorf("Expected a PathError, got %v", err)
		}

		w.Close()
		wg.Wait()

		if werr != nil {
			t.Fatal(werr)
		}
	})

	t.Run("remove with ... when non-recursive", func(t *testing.T) {
		supportsRecurse(t)
		t.Parallel()

		tmp := t.TempDir()
		w := newWatcher(t)
		addWatch(t, w, tmp)

		if err := w.Remove(join(tmp, "...")); err == nil {
			t.Fatal("err was nil")
		}
		if err := w.Remove(tmp); err != nil {
			t.Fatal(err)
		}
	})
}

func TestEventString(t *testing.T) {
	tests := []struct {
		in   Event
		want string
	}{
		{Event{}, `[no events]   ""`},
		{Event{Name: "/file", Op: 0}, `[no events]   "/file"`},

		{Event{Name: "/file", Op: Chmod | Create},
			`CREATE|CHMOD  "/file"`},
		{Event{Name: "/file", Op: Rename},
			`RENAME        "/file"`},
		{Event{Name: "/file", Op: Remove},
			`REMOVE        "/file"`},
		{Event{Name: "/file", Op: Write | Chmod},
			`WRITE|CHMOD   "/file"`},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			have := tt.in.String()
			if have != tt.want {
				t.Errorf("\nhave: %q\nwant: %q", have, tt.want)
			}
		})
	}
}

func TestWatchList(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	file := join(tmp, "file")
	other := join(tmp, "other")

	touch(t, file)
	touch(t, other)

	w := newWatcher(t, file, tmp)
	defer w.Close()

	have := w.WatchList()
	sort.Strings(have)
	want := []string{tmp, file}
	if !reflect.DeepEqual(have, want) {
		t.Errorf("\nhave: %s\nwant: %s", have, want)
	}
}

func TestOpHas(t *testing.T) {
	tests := []struct {
		name string
		o    Op
		h    Op
		want bool
	}{
		{
			name: "single bit match",
			o:    Remove,
			h:    Remove,
			want: true,
		},
		{
			name: "single bit no match",
			o:    Remove,
			h:    Create,
			want: false,
		},
		{
			name: "two bits match",
			o:    Remove | Create,
			h:    Create,
			want: true,
		},
		{
			name: "two bits no match",
			o:    Remove | Create,
			h:    Chmod,
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.o.Has(tt.h); got != tt.want {
				t.Errorf("Has() = %v, want %v", got, tt.want)
			}
		})
	}
}

func BenchmarkWatch(b *testing.B) {
	do := func(b *testing.B, w *Watcher) {
		tmp := b.TempDir()
		file := join(tmp, "file")
		err := w.Add(tmp)
		if err != nil {
			b.Fatal(err)
		}

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			for {
				select {
				case err, ok := <-w.Errors:
					if !ok {
						wg.Done()
						return
					}
					b.Error(err)
				case _, ok := <-w.Events:
					if !ok {
						wg.Done()
						return
					}
				}
			}
		}()

		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			fp, err := os.Create(file)
			if err != nil {
				b.Fatal(err)
			}
			err = fp.Close()
			if err != nil {
				b.Fatal(err)
			}
		}
		err = w.Close()
		if err != nil {
			b.Fatal(err)
		}
		wg.Wait()
	}

	b.Run("default", func(b *testing.B) {
		w, err := NewWatcher()
		if err != nil {
			b.Fatal(err)
		}
		do(b, w)
	})
	b.Run("buffered=1", func(b *testing.B) {
		w, err := NewBufferedWatcher(1)
		if err != nil {
			b.Fatal(err)
		}
		do(b, w)
	})
	b.Run("buffered=1024", func(b *testing.B) {
		w, err := NewBufferedWatcher(1024)
		if err != nil {
			b.Fatal(err)
		}
		do(b, w)
	})
	b.Run("buffered=4096", func(b *testing.B) {
		w, err := NewBufferedWatcher(4096)
		if err != nil {
			b.Fatal(err)
		}
		do(b, w)
	})
}

func BenchmarkAddRemove(b *testing.B) {
	do := func(b *testing.B, w *Watcher) {
		tmp := b.TempDir()
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			if err := w.Add(tmp); err != nil {
				b.Fatal(err)
			}
			if err := w.Remove(tmp); err != nil {
				b.Fatal(err)
			}
		}
	}

	b.Run("default", func(b *testing.B) {
		w, err := NewWatcher()
		if err != nil {
			b.Fatal(err)
		}
		do(b, w)
	})
	b.Run("buffered=1", func(b *testing.B) {
		w, err := NewBufferedWatcher(1)
		if err != nil {
			b.Fatal(err)
		}
		do(b, w)
	})
	b.Run("buffered=1024", func(b *testing.B) {
		w, err := NewBufferedWatcher(1024)
		if err != nil {
			b.Fatal(err)
		}
		do(b, w)
	})
	b.Run("buffered=4096", func(b *testing.B) {
		w, err := NewBufferedWatcher(4096)
		if err != nil {
			b.Fatal(err)
		}
		do(b, w)
	})
}

func TestRace(t *testing.T) {
	// Would panic on inotify: https://github.com/fsnotify/fsnotify/issues/616
	t.Run("add and remove watches", func(t *testing.T) {
		t.Parallel()

		tmp := t.TempDir()
		w := newCollector(t, tmp)
		w.collect(t)

		var (
			dir = join(tmp, "/dir")
			wg  sync.WaitGroup
		)
		wg.Add(400)
		for i := 0; i < 100; i++ {
			go func() { defer wg.Done(); os.MkdirAll(dir, 0o0755) }()
			go func() { defer wg.Done(); os.RemoveAll(dir) }()
			go func() { defer wg.Done(); w.w.Add(dir) }()
			go func() { defer wg.Done(); w.w.Remove(dir) }()
		}
		wg.Wait()
		w.stop(t)
	})

	// Race when deleting watched directory, creating it again, and re-adding
	// it.
	t.Run("remove self", func(t *testing.T) {
		t.Parallel()

		// TODO: seems to hang forever on Windows; possibly related to:
		// https://github.com/fsnotify/fsnotify/issues/656
		//
		// Although it seems to be on different points:
		//
		//   goroutine 8 [chan receive]:
		//   github.com/fsnotify/fsnotify.(*readDirChangesW).AddWith(0xc00002ea40, {0xc0000940f0, 0x48}, {0x0, 0x0, 0xaf2e7f?})
		//           C:/Users/martin/fsnotify/backend_windows.go:141 +0x38d
		//   github.com/fsnotify/fsnotify.(*readDirChangesW).Add(0xc000092000?, {0xc0000940f0?, 0x1?})
		//           C:/Users/martin/fsnotify/backend_windows.go:111 +0x1f
		//   github.com/fsnotify/fsnotify.(*Watcher).Add(...)
		//           C:/Users/martin/fsnotify/fsnotify.go:313
		//   github.com/fsnotify/fsnotify.TestRace.func2(0xc000003a40)
		//           C:/Users/martin/fsnotify/fsnotify_test.go:865 +0x1ee
		//   testing.tRunner(0xc000003a40, 0xb71930)
		//           C:/Program Files/Go/src/testing/testing.go:1792 +0xcb
		//   created by testing.(*T).Run in goroutine 7
		//           C:/Program Files/Go/src/testing/testing.go:1851 +0x3f6
		//
		//   goroutine 9 [select, locked to thread]:
		//   github.com/fsnotify/fsnotify.(*readDirChangesW).sendError(...)
		//           C:/Users/martin/fsnotify/backend_windows.go:85
		//   github.com/fsnotify/fsnotify.(*readDirChangesW).startRead(0xc00002ea40, 0xc0001a4100)
		//           C:/Users/martin/fsnotify/backend_windows.go:453 +0x319
		//   github.com/fsnotify/fsnotify.(*readDirChangesW).readEvents(0xc00002ea40)
		//           C:/Users/martin/fsnotify/backend_windows.go:548 +0x290
		//   created by github.com/fsnotify/fsnotify.newBufferedBackend in goroutine 8
		//           C:/Users/martin/fsnotify/backend_windows.go:55 +0x198
		//
		//   goroutine 18 [chan send]:
		//   github.com/fsnotify/fsnotify.(*eventCollector).collect.func1()
		//           C:/Users/martin/fsnotify/helpers_test.go:436 +0x2cd
		//   created by github.com/fsnotify/fsnotify.(*eventCollector).collect in goroutine 8
		//          C:/Users/martin/fsnotify/helpers_test.go:427 +0x67
		//
		// The Windows backend hasn't really changed in a long time, so old
		// problem, and not something we need to fix right now.
		if runtime.GOOS == "windows" {
			t.Skip("hangs on windows")
		}

		// TODO: sometimes hands on "unix.Close(info.wd)" in kqueue.remove().
		// Only seems to happen on macOS and not the other kqueue platforms.
		//
		// Just skip for now; want to rewrite kqueue backend anyway...
		if runtime.GOOS == "darwin" {
			t.Skip("hangs on macOS")
		}

		tmp := t.TempDir()
		w := newCollector(t, tmp)
		w.collect(t)

		var (
			dir = join(tmp, "/dir")
			wg  sync.WaitGroup
		)
		w.w.Add(dir)
		wg.Add(2000)
		for i := 0; i < 1000; i++ {
			go func() { defer wg.Done(); os.RemoveAll(dir) }()
			go func() { defer wg.Done(); os.MkdirAll(dir, 0o0755) }()
			w.w.Add(dir)
		}
		wg.Wait()
		w.stop(t)
	})
}

func TestNewWatcher(t *testing.T) {
	w, err := NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	defaultSz := 0
	if runtime.GOOS == "windows" {
		defaultSz = 50
	}
	if c := cap(w.Events); c != defaultSz {
		t.Errorf("cap of NewWatcher() is not %d but %d", defaultSz, c)
	}

	w, err = NewBufferedWatcher(0)
	if err != nil {
		t.Fatal(err)
	}
	if c := cap(w.Events); c != 0 {
		t.Errorf("cap of NewWatcher() is not %d but %d", 0, c)
	}

	w, err = NewBufferedWatcher(42)
	if err != nil {
		t.Fatal(err)
	}
	if c := cap(w.Events); c != 42 {
		t.Errorf("cap of NewWatcher() is not %d but %d", 42, c)
	}
}
