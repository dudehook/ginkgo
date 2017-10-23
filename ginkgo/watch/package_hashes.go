package watch

import (
	"path/filepath"
	"regexp"
	"sync"
)

type PackageHashes struct {
	PackageHashes map[string]*PackageHash
	usedPaths     map[string]bool
	watchRegExp   *regexp.Regexp
	lock          *sync.Mutex
	notifier      *Notifier
}

func NewPackageHashes(watchRegExp *regexp.Regexp, notifier *Notifier) *PackageHashes {
	return &PackageHashes{
		PackageHashes: map[string]*PackageHash{},
		usedPaths:     nil,
		watchRegExp:   watchRegExp,
		lock:          &sync.Mutex{},
		notifier:      notifier,
	}
}

func (p *PackageHashes) CheckForChanges() []string {
	p.lock.Lock()
	defer p.lock.Unlock()

	modified := []string{}

	for _, packageHash := range p.PackageHashes {
		if packageHash.CheckForChanges() {
			modified = append(modified, packageHash.path)
		}
	}

	return modified
}

func (p *PackageHashes) Add(path string) *PackageHash {
	p.lock.Lock()
	defer p.lock.Unlock()

	path, _ = filepath.Abs(path)
	_, ok := p.PackageHashes[path]
	if !ok {
		ph := NewPackageHash(path, p.watchRegExp, p.notifier)
		p.PackageHashes[path] = ph
		if p.notifier.enabled {
			// TODO figure out how to get the notification channels percolated up to the top...
			p.notifier.updateChan <- ph.changeNotification
		}
	}

	if p.usedPaths != nil {
		p.usedPaths[path] = true
	}
	return p.PackageHashes[path]
}

func (p *PackageHashes) Get(path string) *PackageHash {
	p.lock.Lock()
	defer p.lock.Unlock()

	path, _ = filepath.Abs(path)
	if p.usedPaths != nil {
		p.usedPaths[path] = true
	}
	return p.PackageHashes[path]
}

func (p *PackageHashes) StartTrackingUsage() {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.usedPaths = map[string]bool{}
}

func (p *PackageHashes) StopTrackingUsageAndPrune() {
	p.lock.Lock()
	defer p.lock.Unlock()

	for path := range p.PackageHashes {
		if !p.usedPaths[path] {
			delete(p.PackageHashes, path)
		}
	}

	p.usedPaths = nil
}
