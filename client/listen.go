package client

import (
	"fmt"
	v1 "github.com/fatedier/frp/pkg/config/v1"
	"github.com/fatedier/frp/pkg/util/log"
	"github.com/samber/lo"
	"github.com/shirou/gopsutil/v4/net"
	"time"
)

func getListenPorts() ([]uint32, error) {
	conns, err := net.Connections("tcp")
	if err != nil {
		return nil, fmt.Errorf("get tcp connections failed: %v", err)
	}
	listenConns := lo.FilterMap(conns, func(item net.ConnectionStat, index int) (uint32, bool) {
		if item.Status == "LISTEN" {
			return item.Laddr.Port, true
		}
		return 0, false
	})
	return lo.Uniq(listenConns), nil
}

func (svr *Service) runListen() {
	if !svr.common.ProxyAllListen {
		return
	}

	excludeListenPort := svr.common.ExcludeListenPort
	otherProxyCfgs := svr.proxyCfgs

	excludeMap := lo.Associate(excludeListenPort, func(port uint32) (uint32, struct{}) {
		return port, struct{}{}
	})
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for _ = range ticker.C {
		ports, err := getListenPorts()
		if err != nil {
			log.Errorf("get current listen ports error: %v", err)
			continue
		}

		ports = lo.Filter(ports, func(port uint32, index int) bool {
			_, exclude := excludeMap[port]
			return port <= 20000 && !exclude
		})

		listenProxyCfgs := lo.Map(ports, func(port uint32, index int) v1.ProxyConfigurer {
			cfg := &v1.TCPProxyConfig{
				ProxyBaseConfig: v1.ProxyBaseConfig{
					Name: fmt.Sprintf("%d", port),
					Type: "tcp",
					ProxyBackend: v1.ProxyBackend{
						LocalPort: int(port),
					},
				},
				RemotePort: int(port),
			}
			cfg.Complete("listen")
			return cfg
		})

		updateCfg := append(otherProxyCfgs, listenProxyCfgs...)
		err = svr.UpdateAllConfigurer(updateCfg, svr.visitorCfgs)
		if err != nil {
			log.Errorf("update all configurers error: %v", err)
		}
	}
}
