/*
CSI Driver for Linstor
Copyright © 2018 LINBIT USA, LLC

This program is free software; you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation; either version 2 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program; if not, see <http://www.gnu.org/licenses/>.
*/

package client

//go:generate go run github.com/vektra/mockery/v2 --srcpkg github.com/LINBIT/golinstor/client --all

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	lapiconsts "github.com/LINBIT/golinstor"
	lapi "github.com/LINBIT/golinstor/client"
	"github.com/LINBIT/golinstor/devicelayerkind"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/piraeusdatastore/linstor-csi/pkg/client/mocks"
	"github.com/piraeusdatastore/linstor-csi/pkg/linstor"
	lc "github.com/piraeusdatastore/linstor-csi/pkg/linstor/highlevelclient"
	"github.com/piraeusdatastore/linstor-csi/pkg/topology"
	"github.com/piraeusdatastore/linstor-csi/pkg/volume"
)

func TestAllocationSizeKiB(t *testing.T) {
	l := &Linstor{}
	tableTests := []struct {
		req int64
		lim int64
		out int64
	}{
		{1024, 0, 4},
		{4096, 4096, 4},
		{4097, 0, 5},
	}

	for _, tt := range tableTests {
		actual, _ := l.AllocationSizeKiB(tt.req, tt.lim)
		if tt.out != actual {
			t.Errorf("Expected: %d, Got: %d, from %+v", tt.out, actual, tt)
		}
	}

	// We'd have to allocate more bytes than the limit since we allocate at KiB
	// Increments.
	_, err := l.AllocationSizeKiB(4097, 40)
	if err == nil {
		t.Errorf("Expected limitBytes to be respected!")
	}
	_, err = l.AllocationSizeKiB(4097, 4096)
	if err == nil {
		t.Errorf("Expected limitBytes to be respected!")
	}
}

func TestValidResourceName(t *testing.T) {
	all := "all"
	if err := validResourceName(all); err == nil {
		t.Fatalf("Expected '%s' to be be an invalid keyword", all)
	}

	tooLong := "abcdefghijklmnopqrstuvwyzABCDEFGHIJKLMNOPQRSTUVWXYZ_______" // 49
	if err := validResourceName(tooLong); err == nil {
		t.Fatalf("Expected '%s' to be too long", tooLong)
	}

	utf8rune := "hello🐱kitty"
	if err := validResourceName(utf8rune); err == nil {
		t.Fatalf("Expected '%s' to fail, because of an utf rune", utf8rune)
	}

	invalid := "_-"
	if err := validResourceName(invalid); err == nil {
		t.Fatalf("Expected '%s' to fail, because it is an invalid name", invalid)
	}

	valid := "rck23"
	if err := validResourceName(valid); err != nil {
		t.Fatalf("Expected '%s' to be valid", valid)
	}
}

func TestLinstorifyResourceName(t *testing.T) {
	unitTests := []struct {
		in, out string
		errExp  bool
	}{
		{
			in:     "rck23",
			out:    "rck23",
			errExp: false,
		}, {
			in:     "hello🐱kitty",
			out:    "hello_kitty",
			errExp: false,
		}, {
			in:     "1be00fd3-d435-436f-be20-561418c62762",
			out:    "LS_1be00fd3-d435-436f-be20-561418c62762",
			errExp: false,
		}, {
			in:     "b1e00fd3-d435-436f-be20-561418c62762",
			out:    "b1e00fd3-d435-436f-be20-561418c62762",
			errExp: false,
		}, {
			in:     "abcdefghijklmnopqrstuvwyzABCDEFGHIJKLMNOPQRSTUVWXYZ_______", // 49
			out:    "should fail",
			errExp: true,
		},
	}

	for _, test := range unitTests {
		resName, err := linstorifyResourceName(test.in)
		switch {
		case test.errExp && err == nil:
			t.Fatalf("Expected that rest '%s' returns an error\n", test.in)
		case !test.errExp && err != nil:
			t.Fatalf("Expected that rest '%s' does not return an error\n", test.in)
		case test.errExp && err != nil:
			continue
		}

		if resName != test.out {
			t.Fatalf("Expected that input '%s' transforms to '%s', but got '%s'\n", test.in, test.out, resName)
		}
	}
}

func TestMkfsArgs(t *testing.T) {
	tableTests := []struct {
		opts, source string
		expected     []string
	}{
		{"-K", "/dev/path", []string{"-K", "/dev/path"}},
		{"", "/dev/path", []string{"/dev/path"}},
	}

	for _, tt := range tableTests {
		actual := mkfsArgs(tt.opts, tt.source)
		if !reflect.DeepEqual(tt.expected, actual) {
			t.Errorf("Expected that mkfsArgs(%q, %q) results in\n\t%v\nbut got\n\t%v\n",
				tt.opts, tt.source, tt.expected, actual)
		}
	}
}

const (
	ExampleResourceID                          = "rsc1"
	ResourceViewAllOnline                      = `[{"name":"rsc1","node_name":"node-0","props":{"StorPoolName":"thinpool"},"layer_object":{"children":[{"type":"STORAGE","storage":{"storage_volumes":[{"volume_number":0,"device_path":"/dev/linstor_vg/rsc1_00000","allocated_size_kib":1052672,"usable_size_kib":1052672,"disk_state":"[]"}]}}],"type":"DRBD","drbd":{"drbd_resource_definition":{"peer_slots":7,"al_stripes":1,"al_stripe_size_kib":32,"port":7000,"transport_type":"IP","secret":"6uwedER7tGEifV9WzGMf","down":false},"node_id":0,"peer_slots":7,"al_stripes":1,"al_size":32,"drbd_volumes":[{"drbd_volume_definition":{"volume_number":0,"minor_number":1000},"device_path":"/dev/drbd1000","backing_device":"/dev/linstor_vg/rsc1_00000","allocated_size_kib":1048840,"usable_size_kib":1048576}],"connections":{"node-2":{"connected":true,"message":"Connected"},"node-1":{"connected":true,"message":"Connected"}},"promotion_score":10103,"may_promote":true}},"state":{"in_use":false},"uuid":"88e64cd1-bac2-4ef7-9abc-5f994c49bada","create_timestamp":1623230527247,"volumes":[{"volume_number":0,"storage_pool_name":"thinpool","provider_kind":"LVM_THIN","device_path":"/dev/drbd1000","allocated_size_kib":315,"props":{"Satellite/Device/Symlinks/0":"/dev/drbd/by-disk/linstor_vg/rsc1_00000","Satellite/Device/Symlinks/1":"/dev/drbd/by-res/rsc1/0"},"state":{"disk_state":"UpToDate"},"layer_data_list":[{"type":"DRBD","data":{"drbd_volume_definition":{"volume_number":0,"minor_number":1000},"device_path":"/dev/drbd1000","backing_device":"/dev/linstor_vg/rsc1_00000","allocated_size_kib":1048840,"usable_size_kib":1048576}},{"type":"STORAGE","data":{"volume_number":0,"device_path":"/dev/linstor_vg/rsc1_00000","allocated_size_kib":1052672,"usable_size_kib":1052672,"disk_state":"[]"}}],"uuid":"96069b58-f483-4897-b443-c89cdf7a0e73"}]},{"name":"rsc1","node_name":"node-1","props":{"StorPoolName":"thinpool"},"layer_object":{"children":[{"type":"STORAGE","storage":{"storage_volumes":[{"volume_number":0,"device_path":"/dev/linstor_vg/rsc1_00000","allocated_size_kib":1052672,"usable_size_kib":1052672,"disk_state":"[]"}]}}],"type":"DRBD","drbd":{"drbd_resource_definition":{"peer_slots":7,"al_stripes":1,"al_stripe_size_kib":32,"port":7000,"transport_type":"IP","secret":"6uwedER7tGEifV9WzGMf","down":false},"node_id":1,"peer_slots":7,"al_stripes":1,"al_size":32,"drbd_volumes":[{"drbd_volume_definition":{"volume_number":0,"minor_number":1000},"device_path":"/dev/drbd1000","backing_device":"/dev/linstor_vg/rsc1_00000","allocated_size_kib":1048840,"usable_size_kib":1048576}],"connections":{"node-2":{"connected":true,"message":"Connected"},"node-0":{"connected":true,"message":"Connected"}},"promotion_score":10103,"may_promote":true}},"state":{"in_use":false},"uuid":"f8f3bac3-bb9c-4334-a9d7-c9e24f31feba","create_timestamp":1623230527453,"volumes":[{"volume_number":0,"storage_pool_name":"thinpool","provider_kind":"LVM_THIN","device_path":"/dev/drbd1000","allocated_size_kib":315,"props":{"Satellite/Device/Symlinks/0":"/dev/drbd/by-disk/linstor_vg/rsc1_00000","Satellite/Device/Symlinks/1":"/dev/drbd/by-res/rsc1/0"},"state":{"disk_state":"UpToDate"},"layer_data_list":[{"type":"DRBD","data":{"drbd_volume_definition":{"volume_number":0,"minor_number":1000},"device_path":"/dev/drbd1000","backing_device":"/dev/linstor_vg/rsc1_00000","allocated_size_kib":1048840,"usable_size_kib":1048576}},{"type":"STORAGE","data":{"volume_number":0,"device_path":"/dev/linstor_vg/rsc1_00000","allocated_size_kib":1052672,"usable_size_kib":1052672,"disk_state":"[]"}}],"uuid":"83aeff7b-ee69-4457-82b6-daa677265045"}]},{"name":"rsc1","node_name":"node-2","props":{"StorPoolName":"thinpool"},"layer_object":{"children":[{"type":"STORAGE","storage":{"storage_volumes":[{"volume_number":0,"device_path":"/dev/linstor_vg/rsc1_00000","allocated_size_kib":1052672,"usable_size_kib":1052672,"disk_state":"[]"}]}}],"type":"DRBD","drbd":{"drbd_resource_definition":{"peer_slots":7,"al_stripes":1,"al_stripe_size_kib":32,"port":7000,"transport_type":"IP","secret":"6uwedER7tGEifV9WzGMf","down":false},"node_id":2,"peer_slots":7,"al_stripes":1,"al_size":32,"drbd_volumes":[{"drbd_volume_definition":{"volume_number":0,"minor_number":1000},"device_path":"/dev/drbd1000","backing_device":"/dev/linstor_vg/rsc1_00000","allocated_size_kib":1048840,"usable_size_kib":1048576}],"connections":{"node-1":{"connected":true,"message":"Connected"},"node-0":{"connected":true,"message":"Connected"}},"promotion_score":10103,"may_promote":true}},"state":{"in_use":false},"uuid":"9ef08b80-3152-46a6-b53d-705395414fbe","create_timestamp":1623230527299,"volumes":[{"volume_number":0,"storage_pool_name":"thinpool","provider_kind":"LVM_THIN","device_path":"/dev/drbd1000","allocated_size_kib":315,"props":{"Satellite/Device/Symlinks/0":"/dev/drbd/by-disk/linstor_vg/rsc1_00000","Satellite/Device/Symlinks/1":"/dev/drbd/by-res/rsc1/0"},"state":{"disk_state":"UpToDate"},"layer_data_list":[{"type":"DRBD","data":{"drbd_volume_definition":{"volume_number":0,"minor_number":1000},"device_path":"/dev/drbd1000","backing_device":"/dev/linstor_vg/rsc1_00000","allocated_size_kib":1048840,"usable_size_kib":1048576}},{"type":"STORAGE","data":{"volume_number":0,"device_path":"/dev/linstor_vg/rsc1_00000","allocated_size_kib":1052672,"usable_size_kib":1052672,"disk_state":"[]"}}],"uuid":"a8ba86a0-5114-4fb8-9934-35a75d33342a"}]}]`
	ResourceViewOneOfflineQuorum               = `[{"name":"rsc1","node_name":"node-0","props":{"StorPoolName":"thinpool"},"layer_object":{"children":[{"type":"STORAGE","storage":{"storage_volumes":[{"volume_number":0,"device_path":"/dev/linstor_vg/rsc1_00000","allocated_size_kib":1052672,"usable_size_kib":1052672,"disk_state":"[]"}]}}],"type":"DRBD","drbd":{"drbd_resource_definition":{"peer_slots":7,"al_stripes":1,"al_stripe_size_kib":32,"port":7000,"transport_type":"IP","secret":"6uwedER7tGEifV9WzGMf","down":false},"node_id":0,"peer_slots":7,"al_stripes":1,"al_size":32,"drbd_volumes":[{"drbd_volume_definition":{"volume_number":0,"minor_number":1000},"device_path":"/dev/drbd1000","backing_device":"/dev/linstor_vg/rsc1_00000","allocated_size_kib":1048840,"usable_size_kib":1048576}]}},"uuid":"88e64cd1-bac2-4ef7-9abc-5f994c49bada","create_timestamp":1623230527247,"volumes":[{"volume_number":0,"storage_pool_name":"thinpool","provider_kind":"LVM_THIN","device_path":"/dev/drbd1000","allocated_size_kib":1052672,"props":{"Satellite/Device/Symlinks/0":"/dev/drbd/by-disk/linstor_vg/rsc1_00000","Satellite/Device/Symlinks/1":"/dev/drbd/by-res/rsc1/0"},"layer_data_list":[{"type":"DRBD","data":{"drbd_volume_definition":{"volume_number":0,"minor_number":1000},"device_path":"/dev/drbd1000","backing_device":"/dev/linstor_vg/rsc1_00000","allocated_size_kib":1048840,"usable_size_kib":1048576}},{"type":"STORAGE","data":{"volume_number":0,"device_path":"/dev/linstor_vg/rsc1_00000","allocated_size_kib":1052672,"usable_size_kib":1052672,"disk_state":"[]"}}],"uuid":"96069b58-f483-4897-b443-c89cdf7a0e73"}]},{"name":"rsc1","node_name":"node-1","props":{"StorPoolName":"thinpool"},"layer_object":{"children":[{"type":"STORAGE","storage":{"storage_volumes":[{"volume_number":0,"device_path":"/dev/linstor_vg/rsc1_00000","allocated_size_kib":1052672,"usable_size_kib":1052672,"disk_state":"[]"}]}}],"type":"DRBD","drbd":{"drbd_resource_definition":{"peer_slots":7,"al_stripes":1,"al_stripe_size_kib":32,"port":7000,"transport_type":"IP","secret":"6uwedER7tGEifV9WzGMf","down":false},"node_id":1,"peer_slots":7,"al_stripes":1,"al_size":32,"drbd_volumes":[{"drbd_volume_definition":{"volume_number":0,"minor_number":1000},"device_path":"/dev/drbd1000","backing_device":"/dev/linstor_vg/rsc1_00000","allocated_size_kib":1048840,"usable_size_kib":1048576}],"connections":{"node-2":{"connected":true,"message":"Connected"},"node-0":{"connected":false,"message":"Connecting"}},"promotion_score":10102,"may_promote":true}},"state":{"in_use":false},"uuid":"f8f3bac3-bb9c-4334-a9d7-c9e24f31feba","create_timestamp":1623230527453,"volumes":[{"volume_number":0,"storage_pool_name":"thinpool","provider_kind":"LVM_THIN","device_path":"/dev/drbd1000","allocated_size_kib":315,"props":{"Satellite/Device/Symlinks/0":"/dev/drbd/by-disk/linstor_vg/rsc1_00000","Satellite/Device/Symlinks/1":"/dev/drbd/by-res/rsc1/0"},"state":{"disk_state":"UpToDate"},"layer_data_list":[{"type":"DRBD","data":{"drbd_volume_definition":{"volume_number":0,"minor_number":1000},"device_path":"/dev/drbd1000","backing_device":"/dev/linstor_vg/rsc1_00000","allocated_size_kib":1048840,"usable_size_kib":1048576}},{"type":"STORAGE","data":{"volume_number":0,"device_path":"/dev/linstor_vg/rsc1_00000","allocated_size_kib":1052672,"usable_size_kib":1052672,"disk_state":"[]"}}],"uuid":"83aeff7b-ee69-4457-82b6-daa677265045"}]},{"name":"rsc1","node_name":"node-2","props":{"StorPoolName":"thinpool"},"layer_object":{"children":[{"type":"STORAGE","storage":{"storage_volumes":[{"volume_number":0,"device_path":"/dev/linstor_vg/rsc1_00000","allocated_size_kib":1052672,"usable_size_kib":1052672,"disk_state":"[]"}]}}],"type":"DRBD","drbd":{"drbd_resource_definition":{"peer_slots":7,"al_stripes":1,"al_stripe_size_kib":32,"port":7000,"transport_type":"IP","secret":"6uwedER7tGEifV9WzGMf","down":false},"node_id":2,"peer_slots":7,"al_stripes":1,"al_size":32,"drbd_volumes":[{"drbd_volume_definition":{"volume_number":0,"minor_number":1000},"device_path":"/dev/drbd1000","backing_device":"/dev/linstor_vg/rsc1_00000","allocated_size_kib":1048840,"usable_size_kib":1048576}],"connections":{"node-1":{"connected":true,"message":"Connected"},"node-0":{"connected":false,"message":"Connecting"}},"promotion_score":10102,"may_promote":true}},"state":{"in_use":false},"uuid":"9ef08b80-3152-46a6-b53d-705395414fbe","create_timestamp":1623230527299,"volumes":[{"volume_number":0,"storage_pool_name":"thinpool","provider_kind":"LVM_THIN","device_path":"/dev/drbd1000","allocated_size_kib":315,"props":{"Satellite/Device/Symlinks/0":"/dev/drbd/by-disk/linstor_vg/rsc1_00000","Satellite/Device/Symlinks/1":"/dev/drbd/by-res/rsc1/0"},"state":{"disk_state":"UpToDate"},"layer_data_list":[{"type":"DRBD","data":{"drbd_volume_definition":{"volume_number":0,"minor_number":1000},"device_path":"/dev/drbd1000","backing_device":"/dev/linstor_vg/rsc1_00000","allocated_size_kib":1048840,"usable_size_kib":1048576}},{"type":"STORAGE","data":{"volume_number":0,"device_path":"/dev/linstor_vg/rsc1_00000","allocated_size_kib":1052672,"usable_size_kib":1052672,"disk_state":"[]"}}],"uuid":"a8ba86a0-5114-4fb8-9934-35a75d33342a"}]}]`
	ResourceViewOneOfflineNoQuorum             = `[{"name":"rsc1","node_name":"node-0","props":{"StorPoolName":"thinpool"},"layer_object":{"children":[{"type":"STORAGE","storage":{"storage_volumes":[{"volume_number":0,"device_path":"/dev/linstor_vg/rsc1_00000","allocated_size_kib":1052672,"usable_size_kib":1052672,"disk_state":"[]"}]}}],"type":"DRBD","drbd":{"drbd_resource_definition":{"peer_slots":7,"al_stripes":1,"al_stripe_size_kib":32,"port":7000,"transport_type":"IP","secret":"6uwedER7tGEifV9WzGMf","down":false},"node_id":0,"peer_slots":7,"al_stripes":1,"al_size":32,"drbd_volumes":[{"drbd_volume_definition":{"volume_number":0,"minor_number":1000},"device_path":"/dev/drbd1000","backing_device":"/dev/linstor_vg/rsc1_00000","allocated_size_kib":1048840,"usable_size_kib":1048576}]}},"uuid":"88e64cd1-bac2-4ef7-9abc-5f994c49bada","create_timestamp":1623230527247,"volumes":[{"volume_number":0,"storage_pool_name":"thinpool","provider_kind":"LVM_THIN","device_path":"/dev/drbd1000","allocated_size_kib":1052672,"props":{"Satellite/Device/Symlinks/0":"/dev/drbd/by-disk/linstor_vg/rsc1_00000","Satellite/Device/Symlinks/1":"/dev/drbd/by-res/rsc1/0"},"layer_data_list":[{"type":"DRBD","data":{"drbd_volume_definition":{"volume_number":0,"minor_number":1000},"device_path":"/dev/drbd1000","backing_device":"/dev/linstor_vg/rsc1_00000","allocated_size_kib":1048840,"usable_size_kib":1048576}},{"type":"STORAGE","data":{"volume_number":0,"device_path":"/dev/linstor_vg/rsc1_00000","allocated_size_kib":1052672,"usable_size_kib":1052672,"disk_state":"[]"}}],"uuid":"96069b58-f483-4897-b443-c89cdf7a0e73"}]},{"name":"rsc1","node_name":"node-1","props":{"StorPoolName":"thinpool"},"layer_object":{"children":[{"type":"STORAGE","storage":{"storage_volumes":[{"volume_number":0,"device_path":"/dev/linstor_vg/rsc1_00000","allocated_size_kib":1052672,"usable_size_kib":1052672,"disk_state":"[]"}]}}],"type":"DRBD","drbd":{"drbd_resource_definition":{"peer_slots":7,"al_stripes":1,"al_stripe_size_kib":32,"port":7000,"transport_type":"IP","secret":"6uwedER7tGEifV9WzGMf","down":false},"node_id":1,"peer_slots":7,"al_stripes":1,"al_size":32,"drbd_volumes":[{"drbd_volume_definition":{"volume_number":0,"minor_number":1000},"device_path":"/dev/drbd1000","backing_device":"/dev/linstor_vg/rsc1_00000","allocated_size_kib":1048840,"usable_size_kib":1048576}],"connections":{"node-2":{"connected":true,"message":"Connected"},"node-0":{"connected":false,"message":"Connecting"}},"promotion_score":10101,"may_promote":true}},"state":{"in_use":false},"uuid":"f8f3bac3-bb9c-4334-a9d7-c9e24f31feba","create_timestamp":1623230527453,"volumes":[{"volume_number":0,"storage_pool_name":"thinpool","provider_kind":"LVM_THIN","device_path":"/dev/drbd1000","allocated_size_kib":315,"props":{"Satellite/Device/Symlinks/0":"/dev/drbd/by-res/rsc1/0","Satellite/Device/Symlinks/1":"/dev/drbd/by-disk/linstor_vg/rsc1_00000"},"state":{"disk_state":"UpToDate"},"layer_data_list":[{"type":"DRBD","data":{"drbd_volume_definition":{"volume_number":0,"minor_number":1000},"device_path":"/dev/drbd1000","backing_device":"/dev/linstor_vg/rsc1_00000","allocated_size_kib":1048840,"usable_size_kib":1048576}},{"type":"STORAGE","data":{"volume_number":0,"device_path":"/dev/linstor_vg/rsc1_00000","allocated_size_kib":1052672,"usable_size_kib":1052672,"disk_state":"[]"}}],"uuid":"83aeff7b-ee69-4457-82b6-daa677265045"}]},{"name":"rsc1","node_name":"node-2","props":{"StorPoolName":"DfltDisklessStorPool"},"flags":["DISKLESS","DRBD_DISKLESS"],"layer_object":{"children":[{"type":"STORAGE","storage":{"storage_volumes":[{"volume_number":0,"allocated_size_kib":0,"usable_size_kib":1048576}]}}],"type":"DRBD","drbd":{"drbd_resource_definition":{"peer_slots":7,"al_stripes":1,"al_stripe_size_kib":32,"port":7000,"transport_type":"IP","secret":"6uwedER7tGEifV9WzGMf","down":false},"node_id":2,"peer_slots":7,"al_stripes":1,"al_size":32,"flags":["DISKLESS"],"drbd_volumes":[{"drbd_volume_definition":{"volume_number":0,"minor_number":1000},"device_path":"/dev/drbd1000","allocated_size_kib":1048840,"usable_size_kib":1048576}],"connections":{"node-1":{"connected":true,"message":"Connected"},"node-0":{"connected":false,"message":"Connecting"}},"promotion_score":1,"may_promote":true}},"state":{"in_use":false},"uuid":"9ef08b80-3152-46a6-b53d-705395414fbe","create_timestamp":1623230527299,"volumes":[{"volume_number":0,"storage_pool_name":"DfltDisklessStorPool","provider_kind":"DISKLESS","device_path":"/dev/drbd1000","allocated_size_kib":0,"props":{"Satellite/Device/Symlinks/0":"/dev/drbd/by-res/rsc1/0","Satellite/Device/Symlinks/1":"/dev/drbd/by-disk/linstor_vg/rsc1_00000"},"state":{"disk_state":"Diskless"},"layer_data_list":[{"type":"DRBD","data":{"drbd_volume_definition":{"volume_number":0,"minor_number":1000},"device_path":"/dev/drbd1000","allocated_size_kib":1048840,"usable_size_kib":1048576}},{"type":"STORAGE","data":{"volume_number":0,"allocated_size_kib":0,"usable_size_kib":1048576}}],"uuid":"a8ba86a0-5114-4fb8-9934-35a75d33342a"}]}]`
	ResourceViewOneDrbdForceDisconnectNoQuorum = `[{"name":"rsc1","node_name":"node-0","props":{"StorPoolName":"thinpool"},"layer_object":{"children":[{"type":"STORAGE","storage":{"storage_volumes":[{"volume_number":0,"device_path":"/dev/linstor_vg/rsc1_00000","allocated_size_kib":1052672,"usable_size_kib":1052672,"disk_state":"[]"}]}}],"type":"DRBD","drbd":{"drbd_resource_definition":{"peer_slots":7,"al_stripes":1,"al_stripe_size_kib":32,"port":7000,"transport_type":"IP","secret":"6uwedER7tGEifV9WzGMf","down":false},"node_id":0,"peer_slots":7,"al_stripes":1,"al_size":32,"drbd_volumes":[{"drbd_volume_definition":{"volume_number":0,"minor_number":1000},"device_path":"/dev/drbd1000","backing_device":"/dev/linstor_vg/rsc1_00000","allocated_size_kib":1048840,"usable_size_kib":1048576}],"connections":{"node-2":{"connected":true,"message":"Connected"},"node-1":{"connected":false,"message":"Connecting"}},"promotion_score":10101,"may_promote":true}},"state":{"in_use":false},"uuid":"88e64cd1-bac2-4ef7-9abc-5f994c49bada","create_timestamp":1623230527247,"volumes":[{"volume_number":0,"storage_pool_name":"thinpool","provider_kind":"LVM_THIN","device_path":"/dev/drbd1000","allocated_size_kib":315,"props":{"Satellite/Device/Symlinks/0":"/dev/drbd/by-res/rsc1/0","Satellite/Device/Symlinks/1":"/dev/drbd/by-disk/linstor_vg/rsc1_00000"},"state":{"disk_state":"UpToDate"},"layer_data_list":[{"type":"DRBD","data":{"drbd_volume_definition":{"volume_number":0,"minor_number":1000},"device_path":"/dev/drbd1000","backing_device":"/dev/linstor_vg/rsc1_00000","allocated_size_kib":1048840,"usable_size_kib":1048576}},{"type":"STORAGE","data":{"volume_number":0,"device_path":"/dev/linstor_vg/rsc1_00000","allocated_size_kib":1052672,"usable_size_kib":1052672,"disk_state":"[]"}}],"uuid":"96069b58-f483-4897-b443-c89cdf7a0e73"}]},{"name":"rsc1","node_name":"node-1","props":{"StorPoolName":"thinpool"},"layer_object":{"children":[{"type":"STORAGE","storage":{"storage_volumes":[{"volume_number":0,"device_path":"/dev/linstor_vg/rsc1_00000","allocated_size_kib":1052672,"usable_size_kib":1052672,"disk_state":"[]"}]}}],"type":"DRBD","drbd":{"drbd_resource_definition":{"peer_slots":7,"al_stripes":1,"al_stripe_size_kib":32,"port":7000,"transport_type":"IP","secret":"6uwedER7tGEifV9WzGMf","down":false},"node_id":1,"peer_slots":7,"al_stripes":1,"al_size":32,"drbd_volumes":[{"drbd_volume_definition":{"volume_number":0,"minor_number":1000},"device_path":"/dev/drbd1000","backing_device":"/dev/linstor_vg/rsc1_00000","allocated_size_kib":1048840,"usable_size_kib":1048576}],"connections":{"node-2":{"connected":false,"message":"StandAlone"},"node-0":{"connected":false,"message":"StandAlone"}},"promotion_score":0,"may_promote":false}},"state":{"in_use":false},"uuid":"f8f3bac3-bb9c-4334-a9d7-c9e24f31feba","create_timestamp":1623230527453,"volumes":[{"volume_number":0,"storage_pool_name":"thinpool","provider_kind":"LVM_THIN","device_path":"/dev/drbd1000","allocated_size_kib":315,"props":{"Satellite/Device/Symlinks/0":"/dev/drbd/by-res/rsc1/0","Satellite/Device/Symlinks/1":"/dev/drbd/by-disk/linstor_vg/rsc1_00000"},"state":{"disk_state":"UpToDate"},"layer_data_list":[{"type":"DRBD","data":{"drbd_volume_definition":{"volume_number":0,"minor_number":1000},"device_path":"/dev/drbd1000","backing_device":"/dev/linstor_vg/rsc1_00000","allocated_size_kib":1048840,"usable_size_kib":1048576}},{"type":"STORAGE","data":{"volume_number":0,"device_path":"/dev/linstor_vg/rsc1_00000","allocated_size_kib":1052672,"usable_size_kib":1052672,"disk_state":"[]"}}],"uuid":"83aeff7b-ee69-4457-82b6-daa677265045"}]},{"name":"rsc1","node_name":"node-2","props":{"StorPoolName":"DfltDisklessStorPool"},"flags":["DISKLESS","DRBD_DISKLESS"],"layer_object":{"children":[{"type":"STORAGE","storage":{"storage_volumes":[{"volume_number":0,"allocated_size_kib":0,"usable_size_kib":1048576}]}}],"type":"DRBD","drbd":{"drbd_resource_definition":{"peer_slots":7,"al_stripes":1,"al_stripe_size_kib":32,"port":7000,"transport_type":"IP","secret":"6uwedER7tGEifV9WzGMf","down":false},"node_id":2,"peer_slots":7,"al_stripes":1,"al_size":32,"flags":["DISKLESS"],"drbd_volumes":[{"drbd_volume_definition":{"volume_number":0,"minor_number":1000},"device_path":"/dev/drbd1000","allocated_size_kib":1048840,"usable_size_kib":1048576}],"connections":{"node-1":{"connected":false,"message":"Connecting"},"node-0":{"connected":true,"message":"Connected"}},"promotion_score":1,"may_promote":true}},"state":{"in_use":false},"uuid":"9ef08b80-3152-46a6-b53d-705395414fbe","create_timestamp":1623230527299,"volumes":[{"volume_number":0,"storage_pool_name":"DfltDisklessStorPool","provider_kind":"DISKLESS","device_path":"/dev/drbd1000","allocated_size_kib":0,"props":{"Satellite/Device/Symlinks/0":"/dev/drbd/by-res/rsc1/0","Satellite/Device/Symlinks/1":"/dev/drbd/by-disk/linstor_vg/rsc1_00000"},"state":{"disk_state":"Diskless"},"layer_data_list":[{"type":"DRBD","data":{"drbd_volume_definition":{"volume_number":0,"minor_number":1000},"device_path":"/dev/drbd1000","allocated_size_kib":1048840,"usable_size_kib":1048576}},{"type":"STORAGE","data":{"volume_number":0,"allocated_size_kib":0,"usable_size_kib":1048576}}],"uuid":"a8ba86a0-5114-4fb8-9934-35a75d33342a"}]}]`
	ResourceViewSharedStoragePool              = `[{"name":"rsc1","node_name":"node-0","props":{"StorPoolName":"shared"},"layer_object":{"children":[{"type":"STORAGE","storage":{"storage_volumes":[{"volume_number":0,"device_path":"/dev/linstor_shared_vg/rsc1_00000","allocated_size_kib":1052672,"usable_size_kib":1052672,"disk_state":"[]"}]}}],"type":"DRBD","drbd":{"drbd_resource_definition":{"peer_slots":7,"al_stripes":1,"al_stripe_size_kib":32,"port":7000,"transport_type":"IP","secret":"QGFBOyTJILQtUyfYgQrq","down":false},"node_id":0,"peer_slots":7,"al_stripes":1,"al_size":32,"flags":["INITIALIZED"],"drbd_volumes":[{"drbd_volume_definition":{"volume_number":0,"minor_number":1000},"device_path":"/dev/drbd1000","backing_device":"/dev/linstor_shared_vg/rsc1_00000","allocated_size_kib":1048840,"usable_size_kib":1048576}],"promotion_score":10101,"may_promote":true}},"state":{"in_use":false},"uuid":"c448d3d8-2278-4840-a982-80c1e425e40f","create_timestamp":1623240635046,"volumes":[{"volume_number":0,"storage_pool_name":"shared","provider_kind":"LVM","device_path":"/dev/drbd1000","allocated_size_kib":1052672,"props":{"Satellite/Device/Symlinks/0":"/dev/drbd/by-res/rsc1/0","Satellite/Device/Symlinks/1":"/dev/drbd/by-disk/linstor_shared_vg/rsc1_00000"},"state":{"disk_state":"UpToDate"},"layer_data_list":[{"type":"DRBD","data":{"drbd_volume_definition":{"volume_number":0,"minor_number":1000},"device_path":"/dev/drbd1000","backing_device":"/dev/linstor_shared_vg/rsc1_00000","allocated_size_kib":1048840,"usable_size_kib":1048576}},{"type":"STORAGE","data":{"volume_number":0,"device_path":"/dev/linstor_shared_vg/rsc1_00000","allocated_size_kib":1052672,"usable_size_kib":1052672,"disk_state":"[]"}}],"uuid":"81d1b9b4-2138-4326-b541-168c59daddc4"}],"shared_name":"shared"},{"name":"rsc1","node_name":"node-1","props":{"StorPoolName":"shared"},"flags":["INACTIVE"],"layer_object":{"children":[{"type":"STORAGE","storage":{"storage_volumes":[{"volume_number":0,"allocated_size_kib":1052672,"usable_size_kib":1052672,"disk_state":"[]"}]}}],"type":"DRBD","drbd":{"drbd_resource_definition":{"peer_slots":7,"al_stripes":1,"al_stripe_size_kib":32,"port":7000,"transport_type":"IP","secret":"QGFBOyTJILQtUyfYgQrq","down":false},"node_id":0,"peer_slots":7,"al_stripes":1,"al_size":32,"flags":["INITIALIZED"],"drbd_volumes":[{"drbd_volume_definition":{"volume_number":0,"minor_number":1000},"allocated_size_kib":1048840,"usable_size_kib":1048576}]}},"state":{},"uuid":"e4c40a9a-2c28-41f9-8714-d6ef0fab4e4c","create_timestamp":1623240634461,"volumes":[{"volume_number":0,"storage_pool_name":"shared","provider_kind":"LVM","allocated_size_kib":1052672,"layer_data_list":[{"type":"DRBD","data":{"drbd_volume_definition":{"volume_number":0,"minor_number":1000},"allocated_size_kib":1048840,"usable_size_kib":1048576}},{"type":"STORAGE","data":{"volume_number":0,"allocated_size_kib":1052672,"usable_size_kib":1052672,"disk_state":"[]"}}],"uuid":"4508140a-b6de-4de9-a3c2-9b9d45b7cc3b"}],"shared_name":"shared"},{"name":"rsc1","node_name":"node-2","props":{"StorPoolName":"shared"},"flags":["INACTIVE"],"layer_object":{"children":[{"type":"STORAGE","storage":{"storage_volumes":[{"volume_number":0,"allocated_size_kib":1052672,"usable_size_kib":1052672,"disk_state":"[]"}]}}],"type":"DRBD","drbd":{"drbd_resource_definition":{"peer_slots":7,"al_stripes":1,"al_stripe_size_kib":32,"port":7000,"transport_type":"IP","secret":"QGFBOyTJILQtUyfYgQrq","down":false},"node_id":0,"peer_slots":7,"al_stripes":1,"al_size":32,"flags":["INITIALIZED"],"drbd_volumes":[{"drbd_volume_definition":{"volume_number":0,"minor_number":1000},"allocated_size_kib":1048840,"usable_size_kib":1048576}]}},"state":{},"uuid":"fb4b32fb-65cc-4c73-9000-b4c4ee8ec2de","create_timestamp":1623240634019,"volumes":[{"volume_number":0,"storage_pool_name":"shared","provider_kind":"LVM","allocated_size_kib":1052672,"layer_data_list":[{"type":"DRBD","data":{"drbd_volume_definition":{"volume_number":0,"minor_number":1000},"allocated_size_kib":1048840,"usable_size_kib":1048576}},{"type":"STORAGE","data":{"volume_number":0,"allocated_size_kib":1052672,"usable_size_kib":1052672,"disk_state":"[]"}}],"uuid":"d1f5247e-251b-4b7e-8ef3-964e2a5a9724"}],"shared_name":"shared"}]`
)

func TestLinstor_Attach(t *testing.T) {
	fromJson := func(s string) ([]lapi.ResourceWithVolumes, error) {
		var result []lapi.ResourceWithVolumes

		err := json.Unmarshal([]byte(s), &result)
		if err != nil {
			return nil, err
		}

		return result, nil
	}

	t.Run("existing resource", func(t *testing.T) {
		m := mocks.ResourceProvider{}
		m.On("GetResourceView", mock.Anything, mock.Anything).Return(fromJson(ResourceViewAllOnline))
		cl := Linstor{client: &lc.HighLevelClient{Client: &lapi.Client{Resources: &m}}, log: logrus.WithField("test", t.Name())}

		err := cl.Attach(context.Background(), &volume.Info{ID: ExampleResourceID, Parameters: map[string]string{"placementcount": "2"}}, "node-2")
		assert.NoError(t, err)
		m.AssertCalled(t, "GetResourceView", mock.Anything, &lapi.ListOpts{Resource: []string{ExampleResourceID}})
		m.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
		m.AssertNotCalled(t, "Get", mock.Anything, mock.Anything, mock.Anything)
		m.AssertNotCalled(t, "Activate", mock.Anything, mock.Anything, mock.Anything)
		m.AssertNotCalled(t, "Deactivate", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("no resource with expected diskfull resources", func(t *testing.T) {
		expectedCreate := lapi.ResourceCreate{
			Resource:  lapi.Resource{Name: ExampleResourceID, NodeName: "node-3", Props: map[string]string{linstor.PropertyCreatedFor: linstor.CreatedForTemporaryDisklessAttach, lapiconsts.KeyStorPoolName: volume.DefaultDisklessStoragePoolName}, Flags: []string{lapiconsts.FlagDrbdDiskless}},
			LayerList: []devicelayerkind.DeviceLayerKind{devicelayerkind.Drbd, devicelayerkind.Storage},
		}
		expectedGetResult := lapi.Resource{}

		m := mocks.ResourceProvider{}
		m.On("GetResourceView", mock.Anything, mock.Anything).Return(fromJson(ResourceViewOneOfflineQuorum))
		m.On("Create", mock.Anything, mock.Anything).Return(nil)
		m.On("Get", mock.Anything, ExampleResourceID, "node-3").Return(expectedGetResult, nil)
		cl := Linstor{client: &lc.HighLevelClient{Client: &lapi.Client{Resources: &m}}, log: logrus.WithField("test", t.Name())}

		err := cl.Attach(context.Background(), &volume.Info{ID: ExampleResourceID, Parameters: map[string]string{"placementcount": "2"}}, "node-3")
		assert.NoError(t, err)
		m.AssertCalled(t, "GetResourceView", mock.Anything, &lapi.ListOpts{Resource: []string{ExampleResourceID}})
		m.AssertCalled(t, "Create", mock.Anything, expectedCreate)
		m.AssertCalled(t, "Get", mock.Anything, ExampleResourceID, "node-3")
		m.AssertNotCalled(t, "Activate", mock.Anything, mock.Anything, mock.Anything)
		m.AssertNotCalled(t, "Deactivate", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("no resource with reduced diskfull resources", func(t *testing.T) {
		expectedCreate := lapi.ResourceCreate{
			Resource:  lapi.Resource{Name: ExampleResourceID, NodeName: "node-3", Props: map[string]string{linstor.PropertyCreatedFor: linstor.CreatedForTemporaryDisklessAttach}, Flags: []string{}},
			LayerList: []devicelayerkind.DeviceLayerKind{devicelayerkind.Drbd, devicelayerkind.Storage},
		}
		expectedGetResult := lapi.Resource{}

		m := mocks.ResourceProvider{}
		m.On("GetResourceView", mock.Anything, mock.Anything).Return(fromJson(ResourceViewOneOfflineNoQuorum))
		m.On("Create", mock.Anything, mock.Anything).Return(nil)
		m.On("Get", mock.Anything, ExampleResourceID, "node-3").Return(expectedGetResult, nil)
		cl := Linstor{client: &lc.HighLevelClient{Client: &lapi.Client{Resources: &m}}, log: logrus.WithField("test", t.Name())}

		err := cl.Attach(context.Background(), &volume.Info{ID: ExampleResourceID, Parameters: map[string]string{"placementcount": "2"}}, "node-3")
		assert.NoError(t, err)
		m.AssertCalled(t, "GetResourceView", mock.Anything, &lapi.ListOpts{Resource: []string{ExampleResourceID}})
		m.AssertCalled(t, "Create", mock.Anything, expectedCreate)
		m.AssertCalled(t, "Get", mock.Anything, ExampleResourceID, "node-3")
		m.AssertNotCalled(t, "Activate", mock.Anything, mock.Anything, mock.Anything)
		m.AssertNotCalled(t, "Deactivate", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("no resource with reduced standalone resources", func(t *testing.T) {
		expectedCreate := lapi.ResourceCreate{
			Resource:  lapi.Resource{Name: ExampleResourceID, NodeName: "node-3", Props: map[string]string{linstor.PropertyCreatedFor: linstor.CreatedForTemporaryDisklessAttach}, Flags: []string{}},
			LayerList: []devicelayerkind.DeviceLayerKind{devicelayerkind.Drbd, devicelayerkind.Storage},
		}
		expectedGetResult := lapi.Resource{}

		m := mocks.ResourceProvider{}
		m.On("GetResourceView", mock.Anything, mock.Anything).Return(fromJson(ResourceViewOneDrbdForceDisconnectNoQuorum))
		m.On("Create", mock.Anything, mock.Anything).Return(nil)
		m.On("Get", mock.Anything, ExampleResourceID, "node-3").Return(expectedGetResult, nil)
		cl := Linstor{client: &lc.HighLevelClient{Client: &lapi.Client{Resources: &m}}, log: logrus.WithField("test", t.Name())}

		err := cl.Attach(context.Background(), &volume.Info{ID: ExampleResourceID, Parameters: map[string]string{"placementcount": "2"}}, "node-3")
		assert.NoError(t, err)
		m.AssertCalled(t, "GetResourceView", mock.Anything, &lapi.ListOpts{Resource: []string{ExampleResourceID}})
		m.AssertCalled(t, "Create", mock.Anything, expectedCreate)
		m.AssertCalled(t, "Get", mock.Anything, ExampleResourceID, "node-3")
		m.AssertNotCalled(t, "Activate", mock.Anything, mock.Anything, mock.Anything)
		m.AssertNotCalled(t, "Deactivate", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("existing resource shared storage pool", func(t *testing.T) {
		m := mocks.ResourceProvider{}
		m.On("GetResourceView", mock.Anything, mock.Anything).Return(fromJson(ResourceViewSharedStoragePool))
		m.On("Deactivate", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		m.On("Activate", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		cl := Linstor{client: &lc.HighLevelClient{Client: &lapi.Client{Resources: &m}}, log: logrus.WithField("test", t.Name())}
		err := cl.Attach(context.Background(), &volume.Info{ID: ExampleResourceID, Parameters: map[string]string{"placementcount": "2"}}, "node-2")
		assert.NoError(t, err)
		m.AssertCalled(t, "GetResourceView", mock.Anything, &lapi.ListOpts{Resource: []string{ExampleResourceID}})
		m.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
		m.AssertNotCalled(t, "Get", mock.Anything, mock.Anything)
		m.AssertCalled(t, "Deactivate", mock.Anything, ExampleResourceID, "node-0")
		m.AssertCalled(t, "Activate", mock.Anything, ExampleResourceID, "node-2")
	})
}

func TestLinstor_CapacityBytes(t *testing.T) {
	t.Parallel()

	m := mocks.NodeProvider{}
	m.On("GetStoragePoolView", mock.Anything).Return([]lapi.StoragePool{
		{
			StoragePoolName: "pool-a",
			NodeName:        "node-1",
			ProviderKind:    lapi.LVM_THIN,
			FreeCapacity:    1,
		},
		{
			StoragePoolName: "pool-a",
			NodeName:        "node-2",
			ProviderKind:    lapi.LVM_THIN,
			FreeCapacity:    2,
		},
		{
			StoragePoolName: "pool-b",
			NodeName:        "node-1",
			ProviderKind:    lapi.ZFS_THIN,
			FreeCapacity:    3,
		},
		{
			StoragePoolName: "pool-b",
			NodeName:        "node-2",
			ProviderKind:    lapi.ZFS_THIN,
			FreeCapacity:    4,
		},
	}, nil)

	cl := Linstor{client: &lc.HighLevelClient{Client: &lapi.Client{Nodes: &m}}, log: logrus.WithField("test", t.Name())}

	testcases := []struct {
		name             string
		params           map[string]string
		topology         map[string]string
		expectedCapacity int64
	}{
		{
			name:             "all",
			expectedCapacity: (1 + 2 + 3 + 4) * 1024,
		},
		{
			name: "just node-1",
			topology: map[string]string{
				topology.LinstorNodeKey: "node-1",
			},
			expectedCapacity: (1 + 3) * 1024,
		},
		{
			name: "just node-2",
			topology: map[string]string{
				topology.LinstorNodeKey: "node-2",
			},
			expectedCapacity: (2 + 4) * 1024,
		},
		{
			name: "just pool-a from params",
			params: map[string]string{
				"StoragePool": "pool-a",
			},
			expectedCapacity: (1 + 2) * 1024,
		},
		{
			name: "just pool-b from params",
			params: map[string]string{
				"StoragePool": "pool-b",
			},
			expectedCapacity: (3 + 4) * 1024,
		},
		{
			name: "just pool-a from topology",
			topology: map[string]string{
				topology.LinstorStoragePoolKeyPrefix + "pool-a": topology.LinstorStoragePoolValue,
			},
			expectedCapacity: (1 + 2) * 1024,
		},
		{
			name: "just pool-a + node-1 from topology",
			topology: map[string]string{
				topology.LinstorStoragePoolKeyPrefix + "pool-a": topology.LinstorStoragePoolValue,
				topology.LinstorNodeKey:                         "node-1",
			},
			expectedCapacity: 1 * 1024,
		},
		{
			name: "unknown node",
			topology: map[string]string{
				topology.LinstorNodeKey: "node-unknown",
			},
			expectedCapacity: 0,
		},
	}

	for i := range testcases {
		testcase := &testcases[i]

		t.Run(testcase.name, func(t *testing.T) {
			cap, err := cl.CapacityBytes(context.Background(), testcase.params, testcase.topology)
			assert.NoError(t, err)
			assert.Equal(t, testcase.expectedCapacity, cap)
		})
	}
}
