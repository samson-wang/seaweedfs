package replication

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/url"
	"pkg/storage"
	"pkg/topology"
	"pkg/util"
	"strconv"
)

/*
This package is created to resolve these replica placement issues:
1. growth factor for each replica level, e.g., add 10 volumes for 1 copy, 20 volumes for 2 copies, 30 volumes for 3 copies
2. in time of tight storage, how to reduce replica level
3. optimizing for hot data on faster disk, cold data on cheaper storage,
4. volume allocation for each bucket
*/

type VolumeGrowth struct {
	copy1factor int
	copy2factor int
	copy3factor int
	copyAll     int
}

func NewDefaultVolumeGrowth() *VolumeGrowth {
	return &VolumeGrowth{copy1factor: 7, copy2factor: 6, copy3factor: 3}
}

func (vg *VolumeGrowth) GrowByType(repType storage.ReplicationType, topo *topology.Topology) (int, error) {
	switch repType {
	case storage.Copy00:
		return vg.GrowByCountAndType(vg.copy1factor, repType, topo)
	case storage.Copy10:
		return vg.GrowByCountAndType(vg.copy2factor, repType, topo)
	case storage.Copy20:
		return vg.GrowByCountAndType(vg.copy3factor, repType, topo)
	case storage.Copy01:
		return vg.GrowByCountAndType(vg.copy2factor, repType, topo)
	case storage.Copy11:
		return vg.GrowByCountAndType(vg.copy3factor, repType, topo)
	}
	return 0, errors.New("Unknown Replication Type!")
}
func (vg *VolumeGrowth) GrowByCountAndType(count int, repType storage.ReplicationType, topo *topology.Topology) (counter int, err error) {
	counter = 0
	switch repType {
	case storage.Copy00:
		for i := 0; i < count; i++ {
			if ok, server, vid := topo.RandomlyReserveOneVolume(); ok {
				if err = vg.grow(topo, *vid, repType, server); err == nil {
					counter++
				}
			}
		}
	case storage.Copy10:
		for i := 0; i < count; i++ {
			nl := topology.NewNodeList(topo.Children(), nil)
			picked, ret := nl.RandomlyPickN(2)
			vid := topo.NextVolumeId()
			if ret {
				var servers []*topology.DataNode
				for _, n := range picked {
					if n.FreeSpace() > 0 {
						if ok, server := n.ReserveOneVolume(rand.Intn(n.FreeSpace()), vid); ok {
							servers = append(servers, server)
						}
					}
				}
				if len(servers) == 2 {
					if err = vg.grow(topo, vid, repType, servers[0], servers[1]); err == nil {
						counter++
					}
				}
			}
		}
	case storage.Copy20:
		for i := 0; i < count; i++ {
			nl := topology.NewNodeList(topo.Children(), nil)
			picked, ret := nl.RandomlyPickN(3)
			vid := topo.NextVolumeId()
			if ret {
				var servers []*topology.DataNode
				for _, n := range picked {
					if n.FreeSpace() > 0 {
						if ok, server := n.ReserveOneVolume(rand.Intn(n.FreeSpace()), vid); ok {
							servers = append(servers, server)
						}
					}
				}
				if len(servers) == 3 {
					if err = vg.grow(topo, vid, repType, servers[0], servers[1], servers[2]); err == nil {
						counter++
					}
				}
			}
		}
	case storage.Copy01:
		for i := 0; i < count; i++ {
			//randomly pick one server, and then choose from the same rack
			if ok, server1, vid := topo.RandomlyReserveOneVolume(); ok {
				rack := server1.Parent()
				exclusion := make(map[string]topology.Node)
				exclusion[server1.String()] = server1
				newNodeList := topology.NewNodeList(rack.Children(), exclusion)
				if newNodeList.FreeSpace() > 0 {
					if ok2, server2 := newNodeList.ReserveOneVolume(rand.Intn(newNodeList.FreeSpace()), *vid); ok2 {
						if err = vg.grow(topo, *vid, repType, server1, server2); err == nil {
							counter++
						}
					}
				}
			}
		}
	case storage.Copy11:
		for i := 0; i < count; i++ {
		}
		err = errors.New("Replication Type Not Implemented Yet!")
	}
	return
}
func (vg *VolumeGrowth) grow(topo *topology.Topology, vid storage.VolumeId, repType storage.ReplicationType, servers ...*topology.DataNode) error {
	for _, server := range servers {
		if err := AllocateVolume(server, vid, repType); err == nil {
			vi := storage.VolumeInfo{Id: vid, Size: 0}
			server.AddOrUpdateVolume(vi)
			topo.RegisterVolumeLayout(&vi, server)
			fmt.Println("Created Volume", vid, "on", server)
		} else {
			fmt.Println("Failed to assign", vid, "to", servers)
			return errors.New("Failed to assign " + vid.String())
		}
	}
	return nil
}

type AllocateVolumeResult struct {
	Error string
}

func AllocateVolume(dn *topology.DataNode, vid storage.VolumeId, repType storage.ReplicationType) error {
	values := make(url.Values)
	values.Add("volume", vid.String())
	values.Add("replicationType", repType.String())
	jsonBlob, err := util.Post("http://"+dn.Ip+":"+strconv.Itoa(dn.Port)+"/admin/assign_volume", values)
	if err != nil {
		return err
	}
	var ret AllocateVolumeResult
	if err := json.Unmarshal(jsonBlob, &ret); err != nil {
		return err
	}
	if ret.Error != "" {
		return errors.New(ret.Error)
	}
	return nil
}
