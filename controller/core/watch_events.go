// Copyright (c) 2017 Cisco and/or its affiliates.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package core

import (
	"github.com/ligato/sfc-controller/controller/controlleridxmap"
)

func (plugin *SfcControllerPluginHandler) subscribeForWatchingEvents() {
	// TODO make this async (once we want resync also for ETCD & router reconnect)
	plugin.ramConfigCache.HEs.WatchNameToIdx(plugin.PluginName, func(event *controlleridxmap.HostEntityEvent) {
		//TODO event.Del
		if err := plugin.renderHostEntity(event.Value, true, true); err != nil {
			plugin.Log.Error("error rendering Host Entity: ", err)
		}
	})
	plugin.ramConfigCache.EEs.WatchNameToIdx(plugin.PluginName, func(event *controlleridxmap.ExternalEntityEvent) {
		//TODO event.Del
		if err := plugin.renderExternalEntity(event.Value, true, true); err != nil {
			plugin.Log.Error("error rendering External Entity: ", err)
		}
	})
	plugin.ramConfigCache.SFCs.WatchNameToIdx(plugin.PluginName, func(event *controlleridxmap.SfcEntityEvent) {
		//TODO event.Del
		if err := plugin.renderServiceFunctionEntity(event.Value); err != nil {
			plugin.Log.Error("error rendering SFC Entity: ", err)
		}
	})
}

/*
type async struct {
	cancel context.CancelFunc // cancel can be used to cancel all goroutines and their jobs inside of the plugin
	wg     sync.WaitGroup     // wait group that allows to wait until all goroutines of the plugin have finished
	nbChangeChan
}

// WatchEvents goroutine is used to watch for changes in the northbound configuration & go context
func (plugin *SfcControllerPluginHandler) watchEvents(ctx context.Context) {
	plugin.async.wg.Add(1)
	defer plugin.async.wg.Done()

	for {
		select {
		case dataChngConfigEv := <-plugin.nbChangeChan:
			err := plugin.changeConfigPropagateRequest(dataChngConfigEv)
			dataChngConfigEvx.Done(err)
		case <-ctx.Done():
			plugin.Log.Debug("Stop watching events")
			return
		}
	}
}
*/
