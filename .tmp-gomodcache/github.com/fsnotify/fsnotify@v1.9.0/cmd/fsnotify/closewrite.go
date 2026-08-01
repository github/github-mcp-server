package main

/*
func closeWrite(paths ...string) {
	if len(paths) < 1 {
		exit("must specify at least one path to watch")
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		exit("creating a new watcher: %s", err)
	}
	defer w.Close()

	var (
		op fsnotify.Op
		cw = w.Supports(fsnotify.UnportableCloseWrite)
	)
	if cw {
		op |= fsnotify.UnportableCloseWrite
	} else {
		op |= fsnotify.Create | fsnotify.Write
	}

	go closeWriteLoop(w, cw)

	for _, p := range paths {
		err := w.AddWith(p, fsnotify.WithOps(op))
		if err != nil {
			exit("%q: %s", p, err)
		}
	}

	printTime("ready; press ^C to exit")
	<-make(chan struct{})
}

func closeWriteLoop(w *fsnotify.Watcher, cw bool) {
	var (
		waitFor = 100 * time.Millisecond
		mu      sync.Mutex
		timers  = make(map[string]*time.Timer)
	)
	for {
		select {
		case err, ok := <-w.Errors:
			if !ok {
				return
			}
			panic(err)
		case e, ok := <-w.Events:
			if !ok {
				return
			}

			// CloseWrite is supported: easy case.
			if cw {
				if e.Has(fsnotify.UnportableCloseWrite) {
					printTime(e.String())
				}
				continue
			}

			// Get timer.
			mu.Lock()
			t, ok := timers[e.Name]
			mu.Unlock()

			// No timer yet, so create one.
			if !ok {
				t = time.AfterFunc(math.MaxInt64, func() {
					printTime(e.String())
					mu.Lock()
					delete(timers, e.Name)
					mu.Unlock()
				})
				t.Stop()

				mu.Lock()
				timers[e.Name] = t
				mu.Unlock()
			}

			// Reset the timer for this path, so it will start from 100ms again.
			t.Reset(waitFor)
		}
	}
}
*/
