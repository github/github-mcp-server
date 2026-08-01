package pflag

import (
	"fmt"
	"net"
	"strings"
	"testing"
)

// Helper function to set static slices
func getCIDR(ip net.IP, cidr *net.IPNet, err error) net.IPNet {
	return *cidr
}

func equalCIDR(c1 net.IPNet, c2 net.IPNet) bool {
	if c1.String() == c2.String() {
		return true
	}
	return false
}

func setUpIPNetFlagSet(ipsp *[]net.IPNet) *FlagSet {
	f := NewFlagSet("test", ContinueOnError)
	f.IPNetSliceVar(ipsp, "cidrs", []net.IPNet{}, "Command separated list!")
	return f
}

func setUpIPNetFlagSetWithDefault(ipsp *[]net.IPNet) *FlagSet {
	f := NewFlagSet("test", ContinueOnError)
	f.IPNetSliceVar(ipsp, "cidrs",
		[]net.IPNet{
			getCIDR(net.ParseCIDR("192.168.1.1/16")),
			getCIDR(net.ParseCIDR("fd00::/64")),
		},
		"Command separated list!")
	return f
}

func TestEmptyIPNet(t *testing.T) {
	var cidrs []net.IPNet
	f := setUpIPNetFlagSet(&cidrs)
	err := f.Parse([]string{})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}

	getIPNet, err := f.GetIPNetSlice("cidrs")
	if err != nil {
		t.Fatal("got an error from GetIPNetSlice():", err)
	}
	if len(getIPNet) != 0 {
		t.Fatalf("got ips %v with len=%d but expected length=0", getIPNet, len(getIPNet))
	}
}

func TestIPNets(t *testing.T) {
	var ips []net.IPNet
	f := setUpIPNetFlagSet(&ips)

	vals := []string{"192.168.1.1/24", "10.0.0.1/16", "fd00:0:0:0:0:0:0:2/64"}
	arg := fmt.Sprintf("--cidrs=%s", strings.Join(vals, ","))
	err := f.Parse([]string{arg})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}
	for i, v := range ips {
		if _, cidr, _ := net.ParseCIDR(vals[i]); cidr == nil {
			t.Fatalf("invalid string being converted to CIDR: %s", vals[i])
		} else if !equalCIDR(*cidr, v) {
			t.Fatalf("expected ips[%d] to be %s but got: %s from GetIPSlice", i, vals[i], v)
		}
	}
}

func TestIPNetDefault(t *testing.T) {
	var cidrs []net.IPNet
	f := setUpIPNetFlagSetWithDefault(&cidrs)

	vals := []string{"192.168.1.1/16", "fd00::/64"}
	err := f.Parse([]string{})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}
	for i, v := range cidrs {
		if _, cidr, _ := net.ParseCIDR(vals[i]); cidr == nil {
			t.Fatalf("invalid string being converted to CIDR: %s", vals[i])
		} else if !equalCIDR(*cidr, v) {
			t.Fatalf("expected cidrs[%d] to be %s but got: %s", i, vals[i], v)
		}
	}

	getIPNet, err := f.GetIPNetSlice("cidrs")
	if err != nil {
		t.Fatal("got an error from GetIPNetSlice")
	}
	for i, v := range getIPNet {
		if _, cidr, _ := net.ParseCIDR(vals[i]); cidr == nil {
			t.Fatalf("invalid string being converted to CIDR: %s", vals[i])
		} else if !equalCIDR(*cidr, v) {
			t.Fatalf("expected cidrs[%d] to be %s but got: %s", i, vals[i], v)
		}
	}
}

func TestIPNetWithDefault(t *testing.T) {
	var cidrs []net.IPNet
	f := setUpIPNetFlagSetWithDefault(&cidrs)

	vals := []string{"192.168.1.1/16", "fd00::/64"}
	arg := fmt.Sprintf("--cidrs=%s", strings.Join(vals, ","))
	err := f.Parse([]string{arg})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}
	for i, v := range cidrs {
		if _, cidr, _ := net.ParseCIDR(vals[i]); cidr == nil {
			t.Fatalf("invalid string being converted to CIDR: %s", vals[i])
		} else if !equalCIDR(*cidr, v) {
			t.Fatalf("expected cidrs[%d] to be %s but got: %s", i, vals[i], v)
		}
	}

	getIPNet, err := f.GetIPNetSlice("cidrs")
	if err != nil {
		t.Fatal("got an error from GetIPNetSlice")
	}
	for i, v := range getIPNet {
		if _, cidr, _ := net.ParseCIDR(vals[i]); cidr == nil {
			t.Fatalf("invalid string being converted to CIDR: %s", vals[i])
		} else if !equalCIDR(*cidr, v) {
			t.Fatalf("expected cidrs[%d] to be %s but got: %s", i, vals[i], v)
		}
	}
}

func TestIPNetCalledTwice(t *testing.T) {
	var cidrs []net.IPNet
	f := setUpIPNetFlagSet(&cidrs)

	in := []string{"192.168.1.2/16,fd00::/64", "10.0.0.1/24"}

	expected := []net.IPNet{
		getCIDR(net.ParseCIDR("192.168.1.2/16")),
		getCIDR(net.ParseCIDR("fd00::/64")),
		getCIDR(net.ParseCIDR("10.0.0.1/24")),
	}
	argfmt := "--cidrs=%s"
	arg1 := fmt.Sprintf(argfmt, in[0])
	arg2 := fmt.Sprintf(argfmt, in[1])
	err := f.Parse([]string{arg1, arg2})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}
	for i, v := range cidrs {
		if !equalCIDR(expected[i], v) {
			t.Fatalf("expected cidrs[%d] to be %s but got: %s", i, expected[i], v)
		}
	}
}

func TestIPNetBadQuoting(t *testing.T) {

	tests := []struct {
		Want    []net.IPNet
		FlagArg []string
	}{
		{
			Want: []net.IPNet{
				getCIDR(net.ParseCIDR("a4ab:61d:f03e:5d7d:fad7:d4c2:a1a5:568/128")),
				getCIDR(net.ParseCIDR("203.107.49.208/32")),
				getCIDR(net.ParseCIDR("14.57.204.90/32")),
			},
			FlagArg: []string{
				"a4ab:61d:f03e:5d7d:fad7:d4c2:a1a5:568/128",
				"203.107.49.208/32",
				"14.57.204.90/32",
			},
		},
		{
			Want: []net.IPNet{
				getCIDR(net.ParseCIDR("204.228.73.195/32")),
				getCIDR(net.ParseCIDR("86.141.15.94/32")),
			},
			FlagArg: []string{
				"204.228.73.195/32",
				"86.141.15.94/32",
			},
		},
		{
			Want: []net.IPNet{
				getCIDR(net.ParseCIDR("c70c:db36:3001:890f:c6ea:3f9b:7a39:cc3f/128")),
				getCIDR(net.ParseCIDR("4d17:1d6e:e699:bd7a:88c5:5e7e:ac6a:4472/128")),
			},
			FlagArg: []string{
				"c70c:db36:3001:890f:c6ea:3f9b:7a39:cc3f/128",
				"4d17:1d6e:e699:bd7a:88c5:5e7e:ac6a:4472/128",
			},
		},
		{
			Want: []net.IPNet{
				getCIDR(net.ParseCIDR("5170:f971:cfac:7be3:512a:af37:952c:bc33/128")),
				getCIDR(net.ParseCIDR("93.21.145.140/32")),
				getCIDR(net.ParseCIDR("2cac:61d3:c5ff:6caf:73e0:1b1a:c336:c1ca/128")),
			},
			FlagArg: []string{
				" 5170:f971:cfac:7be3:512a:af37:952c:bc33/128  , 93.21.145.140/32     ",
				"2cac:61d3:c5ff:6caf:73e0:1b1a:c336:c1ca/128",
			},
		},
		{
			Want: []net.IPNet{
				getCIDR(net.ParseCIDR("2e5e:66b2:6441:848:5b74:76ea:574c:3a7b/128")),
				getCIDR(net.ParseCIDR("2e5e:66b2:6441:848:5b74:76ea:574c:3a7b/128")),
				getCIDR(net.ParseCIDR("2e5e:66b2:6441:848:5b74:76ea:574c:3a7b/128")),
				getCIDR(net.ParseCIDR("2e5e:66b2:6441:848:5b74:76ea:574c:3a7b/128")),
			},
			FlagArg: []string{
				`"2e5e:66b2:6441:848:5b74:76ea:574c:3a7b/128,        2e5e:66b2:6441:848:5b74:76ea:574c:3a7b/128,2e5e:66b2:6441:848:5b74:76ea:574c:3a7b/128     "`,
				" 2e5e:66b2:6441:848:5b74:76ea:574c:3a7b/128"},
		},
	}

	for i, test := range tests {

		var cidrs []net.IPNet
		f := setUpIPNetFlagSet(&cidrs)

		if err := f.Parse([]string{fmt.Sprintf("--cidrs=%s", strings.Join(test.FlagArg, ","))}); err != nil {
			t.Fatalf("flag parsing failed with error: %s\nparsing:\t%#v\nwant:\t\t%s",
				err, test.FlagArg, test.Want[i])
		}

		for j, b := range cidrs {
			if !equalCIDR(b, test.Want[j]) {
				t.Fatalf("bad value parsed for test %d on net.IP %d:\nwant:\t%s\ngot:\t%s", i, j, test.Want[j], b)
			}
		}
	}
}
