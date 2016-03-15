package daemon

import (
	"github.com/dotcloud/docker/engine"
	"github.com/dotcloud/docker/nat"
)

// FIXME: move deprecated port stuff to nat to clean up the core.
type PortMapping map[string]string // Deprecated

type NetworkSettings struct {
	IPAddress   string
	IPPrefixLen int
	Gateway     string
	Bridge      string
	PortMapping map[string]PortMapping // Deprecated
	Ports       nat.PortMap
}

func (settings *NetworkSettings) PortMappingAPI() *engine.Table {
	var outs = engine.NewTable("", 0)
	for port, bindings := range settings.Ports {
		p, _ := nat.ParsePort(port.Port())
		if len(bindings) == 0 {
			out := &engine.Env{}
			out.SetInt("PrivatePort", p)
			out.Set("Type", port.Proto())
			outs.Add(out)
			continue
		}
		for _, binding := range bindings {
			out := &engine.Env{}
			h, _ := nat.ParsePort(binding.HostPort)
			out.SetInt("PrivatePort", p)
			out.SetInt("PublicPort", h)
			out.Set("Type", port.Proto())
			out.Set("IP", binding.HostIp)
			outs.Add(out)
		}
	}
	return outs
}
