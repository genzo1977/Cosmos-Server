package docker

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"

	"github.com/azukaar/cosmos-server/src/utils"
	"github.com/docker/docker/api/types"

	"github.com/docker/docker/api/types/mount"

)

func ExportDocker() {
	errD := Connect()
	if errD != nil {
		utils.Error("ExportDocker - connect - ", errD)
		return
	}

	finalBackup := DockerServiceCreateRequest{}
	
	// List containers
	containers, err := DockerClient.ContainerList(DockerContext, types.ContainerListOptions{})
	if err != nil {
		utils.Error("ExportDocker - Cannot list containers", err)
		return
	}

	
	// Convert the containers into your custom format
	var services = make(map[string]ContainerCreateRequestContainer)
	for _, container := range containers {
		// Fetch detailed info of each container
		detailedInfo, err := DockerClient.ContainerInspect(DockerContext, container.ID)
		if err != nil {
			utils.Error("Cannot inspect container", err)
			return
		}

		// Map the detailedInfo to your ContainerCreateRequestContainer struct
		// Here's a simplified example, you'd need to handle all the fields
		service := ContainerCreateRequestContainer{
			Name:         strings.TrimPrefix(detailedInfo.Name, "/"),
			Image:        detailedInfo.Config.Image,
			Environment:  detailedInfo.Config.Env,
			Labels:       detailedInfo.Config.Labels,
			Command:      strings.Join(detailedInfo.Config.Cmd, " "),
			Entrypoint:   strings.Join(detailedInfo.Config.Entrypoint, " "),
			WorkingDir:   detailedInfo.Config.WorkingDir,
			User:         detailedInfo.Config.User,
			Tty:          detailedInfo.Config.Tty,
			StdinOpen:    detailedInfo.Config.OpenStdin,
			Hostname:     detailedInfo.Config.Hostname,
			Domainname:   detailedInfo.Config.Domainname,
			MacAddress:   detailedInfo.NetworkSettings.MacAddress,
			NetworkMode:  string(detailedInfo.HostConfig.NetworkMode),
			StopSignal:   detailedInfo.Config.StopSignal,
			HealthCheck:  ContainerCreateRequestContainerHealthcheck {
			},
			DNS:              detailedInfo.HostConfig.DNS,
			DNSSearch:        detailedInfo.HostConfig.DNSSearch,
			ExtraHosts:       detailedInfo.HostConfig.ExtraHosts,
			SecurityOpt:      detailedInfo.HostConfig.SecurityOpt,
			StorageOpt:       detailedInfo.HostConfig.StorageOpt,
			Sysctls:          detailedInfo.HostConfig.Sysctls,
			Isolation:        string(detailedInfo.HostConfig.Isolation),
			CapAdd:           detailedInfo.HostConfig.CapAdd,
			CapDrop:          detailedInfo.HostConfig.CapDrop,
			SysctlsMap:       detailedInfo.HostConfig.Sysctls,
			Privileged:       detailedInfo.HostConfig.Privileged,
			// StopGracePeriod:  int(detailedInfo.HostConfig.StopGracePeriod.Seconds()),
			// Ports
			Ports: func() []string {
					ports := []string{}
					for port, binding := range detailedInfo.NetworkSettings.Ports {
							for _, b := range binding {
									ports = append(ports, fmt.Sprintf("%s:%s->%s/%s", b.HostIP, b.HostPort, port.Port(), port.Proto()))
							}
					}
					return ports
			}(),
			// Volumes
			Volumes: func() []mount.Mount {
					mounts := []mount.Mount{}
					for _, m := range detailedInfo.Mounts {
						  mount := mount.Mount{
								Type:        m.Type,
								Source:      m.Source,
								Target:      m.Destination,
								ReadOnly:    !m.RW,
								// Consistency: mount.Consistency(m.Consistency),
						}

						if m.Type == "volume" {
							nodata := strings.Split(strings.TrimSuffix(m.Source, "/_data"), "/")
							mount.Source = nodata[len(nodata)-1]
						}

						mounts = append(mounts, mount)
					}
					return mounts
			}(),
			// Networks
			Networks: func() map[string]ContainerCreateRequestServiceNetwork {
					networks := make(map[string]ContainerCreateRequestServiceNetwork)
					for netName, netConfig := range detailedInfo.NetworkSettings.Networks {
							networks[netName] = ContainerCreateRequestServiceNetwork{
									Aliases:     netConfig.Aliases,
									IPV4Address: netConfig.IPAddress,
									IPV6Address: netConfig.GlobalIPv6Address,
							}
					}
					return networks
			}(),

			DependsOn:      []string{},  // This is not directly available from inspect. It's part of docker-compose.
			RestartPolicy:  string(detailedInfo.HostConfig.RestartPolicy.Name),
			Devices:        func() []string {
					var devices []string
					for _, device := range detailedInfo.HostConfig.Devices {
							devices = append(devices, fmt.Sprintf("%s:%s", device.PathOnHost, device.PathInContainer))
					}
					return devices
			}(),
			Expose:         []string{},  // This information might need to be derived from other properties
		}

		// healthcheck
		if detailedInfo.Config.Healthcheck != nil {
			service.HealthCheck.Test = detailedInfo.Config.Healthcheck.Test
			service.HealthCheck.Interval = int(detailedInfo.Config.Healthcheck.Interval.Seconds())
			service.HealthCheck.Timeout = int(detailedInfo.Config.Healthcheck.Timeout.Seconds())
			service.HealthCheck.Retries = detailedInfo.Config.Healthcheck.Retries
			service.HealthCheck.StartPeriod = int(detailedInfo.Config.Healthcheck.StartPeriod.Seconds())
		}

		// user UID/GID
		if detailedInfo.Config.User != "" {
			parts := strings.Split(detailedInfo.Config.User, ":")
			if len(parts) == 2 {
				uid, err := strconv.Atoi(parts[0])
				if err != nil {
					panic(err)
				}
				gid, err := strconv.Atoi(parts[1])
				if err != nil {
					panic(err)
				}
				service.UID = uid
				service.GID = gid
			}
		}

		//expose 
		// for _, port := range detailedInfo.Config.ExposedPorts {
			
		// }
		
		services[strings.TrimPrefix(detailedInfo.Name, "/")] = service
	}

	// List networks
	networks, err := DockerClient.NetworkList(DockerContext, types.NetworkListOptions{})
	if err != nil {
		utils.Error("Cannot list networks", err)
		return
	}

	finalBackup.Networks = make(map[string]ContainerCreateRequestNetwork)

	// Convert the networks into custom format
	for _, network := range networks {
		if network.Name == "bridge" || network.Name == "host" || network.Name == "none" {
			continue
		}

		// Fetch detailed info of each network
		detailedInfo, err := DockerClient.NetworkInspect(DockerContext, network.ID, types.NetworkInspectOptions{})
		if err != nil {
			utils.Error("Cannot inspect network", err)
			return
		}

		// Map the detailedInfo to ContainerCreateRequestContainer struct
		network := ContainerCreateRequestNetwork{
			Name:         detailedInfo.Name,
			Driver:       detailedInfo.Driver,
			Internal:     detailedInfo.Internal,
			Attachable:   detailedInfo.Attachable,
			EnableIPv6:   detailedInfo.EnableIPv6,
			Labels:       detailedInfo.Labels,
		}

		network.IPAM.Driver = detailedInfo.IPAM.Driver
		for _, config := range detailedInfo.IPAM.Config {
			network.IPAM.Config = append(network.IPAM.Config, ContainerCreateRequestNetworkIPAMConfig{
				Subnet:  config.Subnet,
				Gateway: config.Gateway,
			})
		}

		finalBackup.Networks[detailedInfo.Name] = network
	}

	// Convert the services map to your finalBackup struct
	finalBackup.Services = services


	// Convert the finalBackup struct to JSON
	jsonData, err := json.MarshalIndent(finalBackup, "", "  ")
	if err != nil {
		utils.Error("Cannot marshal docker backup", err)
	}

	// Write the JSON data to a file
	err = ioutil.WriteFile(utils.CONFIGFOLDER + "backup.cosmos-compose.json", jsonData, 0644)
	if err != nil {
		utils.Error("Cannot save docker backup", err)
	}
}