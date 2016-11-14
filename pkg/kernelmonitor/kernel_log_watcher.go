/*
Copyright 2016 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kernelmonitor

import (
	"os"
	"strings"
	"time"

	"k8s.io/node-problem-detector/pkg/kernelmonitor/types"
	"k8s.io/node-problem-detector/pkg/kernelmonitor/util"

	"github.com/euank/go-kmsg-parser/kmsgparser"
	"github.com/golang/glog"
	utilclock "github.com/pivotal-golang/clock"
)

// WatcherConfig is the configuration of kernel log watcher.
type WatcherConfig struct {
	// StartPattern is the pattern of the start line
	StartPattern string `json:"startPattern, omitempty"`
	// Lookback is the time kernel watcher looks up
	Lookback string `json:"lookback, omitempty"`
}

// KernelLogWatcher watches the kernel log. Once there is new log line,
// it will translate and report the log.
type KernelLogWatcher interface {
	// Watch starts the kernel log watcher and returns a watch channel.
	Watch() (<-chan *types.KernelLog, error)
	// Stop stops the kernel log watcher.
	Stop()
}

type kernelLogWatcher struct {
	cfg   WatcherConfig
	logCh chan *types.KernelLog
	tomb  *util.Tomb

	// kmsgParser can be used for mocking
	kmsgParser kmsgparser.Parser
	// clock is used for mocking
	clock utilclock.Clock
}

// NewKernelLogWatcher creates a new kernel log watcher.
func NewKernelLogWatcher(cfg WatcherConfig) KernelLogWatcher {
	return &kernelLogWatcher{
		cfg:  cfg,
		tomb: util.NewTomb(),
		// A capacity 1000 buffer should be enough
		logCh: make(chan *types.KernelLog, 1000),
		clock: utilclock.NewClock(),
	}
}

func (k *kernelLogWatcher) Watch() (<-chan *types.KernelLog, error) {
	// NOTE(euank,random-liu): This is a bit of a hack just in case some OS
	// Distro we run doesn't have `/dev/kmsg`. This is really unlikely on linux
	// since its existed since 3.5, but we don't want to crashloop the whole pod
	// just if we can't find this.
	// If we're in a bad config, print a log message and return no error.
	if k.kmsgParser == nil {
		if _, err := os.Stat("/dev/kmsg"); os.IsNotExist(err) {
			glog.Infof("kmsg device '/dev/kmsg' not found, kernel monitor doesn't support this os distro")
			return nil, nil
		}
	}
	go k.watchLoop()
	return k.logCh, nil
}

func (k *kernelLogWatcher) Stop() {
	k.kmsgParser.Close()
	k.tomb.Stop()
}

// watchLoop is the main watch loop of kernel log watcher.
func (k *kernelLogWatcher) watchLoop() {
	if k.kmsgParser == nil {
		parser, err := kmsgparser.NewParser()
		if err != nil {
			glog.Fatalf("failed to create kmsg parser: %v", err)
		}
		k.kmsgParser = parser
	}

	defer func() {
		close(k.logCh)
		k.tomb.Done()
	}()
	lookback, err := parseDuration(k.cfg.Lookback)
	if err != nil {
		glog.Fatalf("failed to parse duration %q: %v", k.cfg.Lookback, err)
	}

	kmsgs := k.kmsgParser.Parse()

	for {
		select {
		case <-k.tomb.Stopping():
			glog.Infof("Stop watching kernel log")
			k.kmsgParser.Close()
			return
		case msg := <-kmsgs:
			glog.V(5).Infof("got kernel message: %+v", msg)
			if msg.Message == "" {
				continue
			}

			// Discard too old messages
			if k.clock.Since(msg.Timestamp) > lookback {
				glog.V(5).Infof("throwing away msg %v for being too old: %v > %v", msg.Message, msg.Timestamp.String(), lookback.String())
				continue
			}

			k.logCh <- &types.KernelLog{
				Message:   strings.TrimSpace(msg.Message),
				Timestamp: msg.Timestamp,
			}
		}
	}
}

func parseDuration(s string) (time.Duration, error) {
	// If the duration is not configured, just return 0 by default
	if s == "" {
		return 0, nil
	}
	return time.ParseDuration(s)
}
