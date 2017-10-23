package watch

import (
	"fmt"
	"reflect"

	"regexp"

	"github.com/onsi/ginkgo/ginkgo/testsuite"
)

type SuiteErrors map[testsuite.TestSuite]error

type Notifier struct {
	enabled    bool
	updateChan chan (chan bool)
}

type DeltaTracker struct {
	maxDepth      int
	watchRegExp   *regexp.Regexp
	suites        map[string]*Suite
	packageHashes *PackageHashes

	ChangeNotification chan bool
	notifier           *Notifier
}

func NewDeltaTracker(maxDepth int, watchRegExp *regexp.Regexp, useFSNotify bool) *DeltaTracker {
	dt := &DeltaTracker{
		maxDepth:    maxDepth,
		watchRegExp: watchRegExp,
		notifier:    &Notifier{enabled: false},
		suites:      map[string]*Suite{},
	}
	if useFSNotify {
		dt.ChangeNotification = make(chan bool, 600)
		dt.setupFSNotifications()
	}
	dt.packageHashes = NewPackageHashes(dt.watchRegExp, dt.notifier)
	return dt
}

func (d *DeltaTracker) setupFSNotifications() {

	d.notifier.updateChan = make(chan (chan bool))
	d.notifier.enabled = true
	go func() {
		var cases []reflect.SelectCase
		cases = append(cases, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(d.notifier.updateChan)})
		for {
			fmt.Println("FSNotification Select...")
			chosen, value, ok := reflect.Select(cases)
			fmt.Println("FSNotification got select.")
			// A channel closed
			if !ok {
				cases[chosen].Chan = reflect.ValueOf(nil)
				fmt.Println("  -- channel closed")
				continue
			}
			// If we get a messaeg on the updateChan, add the value to the select cases
			if chosen == 0 {
				fmt.Println("  -- update chan event, updating")
				cases = append(cases, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: value})
				fmt.Println("  -- update chan event, updated chan list")
				continue
			}
			// Otherwise we got a file change notification
			fmt.Println("  -- file change notification passthrough")
			fmt.Println(chosen)
			fmt.Println(value)
			d.ChangeNotification <- true
		}
	}()
}

func (d *DeltaTracker) Delta(suites []testsuite.TestSuite) (delta Delta, errors SuiteErrors) {
	errors = SuiteErrors{}
	delta.ModifiedPackages = d.packageHashes.CheckForChanges()

	providedSuitePaths := map[string]bool{}
	for _, suite := range suites {
		providedSuitePaths[suite.Path] = true
	}

	d.packageHashes.StartTrackingUsage()

	for _, suite := range d.suites {
		if providedSuitePaths[suite.Suite.Path] {
			if suite.Delta() > 0 {
				delta.modifiedSuites = append(delta.modifiedSuites, suite)
			}
		} else {
			delta.RemovedSuites = append(delta.RemovedSuites, suite)
		}
	}

	d.packageHashes.StopTrackingUsageAndPrune()

	for _, suite := range suites {
		_, ok := d.suites[suite.Path]
		if !ok {
			s, err := NewSuite(suite, d.maxDepth, d.packageHashes)
			if err != nil {
				errors[suite] = err
				continue
			}
			d.suites[suite.Path] = s
			delta.NewSuites = append(delta.NewSuites, s)
		}
	}

	return delta, errors
}

func (d *DeltaTracker) WillRun(suite testsuite.TestSuite) error {
	s, ok := d.suites[suite.Path]
	if !ok {
		return fmt.Errorf("unknown suite %s", suite.Path)
	}

	return s.MarkAsRunAndRecomputedDependencies(d.maxDepth)
}
