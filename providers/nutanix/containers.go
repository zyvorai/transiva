// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"

	clustermgmtapi "github.com/nutanix/ntnx-api-golang-clients/clustermgmt-go-client/v4/api"
	clustermgmtclient "github.com/nutanix/ntnx-api-golang-clients/clustermgmt-go-client/v4/client"
	clusterconfig "github.com/nutanix/ntnx-api-golang-clients/clustermgmt-go-client/v4/models/clustermgmt/v4/config"
)

func newClusterMgmtClient(cfg *ClientConfig) *clustermgmtclient.ApiClient {
	port := cfg.Port
	if port == 0 {
		port = 9440
	}
	apiClient := clustermgmtclient.NewApiClient()
	apiClient.Host = cfg.Host
	apiClient.Port = port
	apiClient.Username = cfg.Username
	apiClient.Password = cfg.Password
	apiClient.VerifySSL = cfg.VerifySSL
	return apiClient
}

// ResolveContainerNames lists storage containers and returns UUID-to-name mapping.
func (c *Client) ResolveContainerNames(ctx context.Context) (map[string]string, error) {
	api := clustermgmtapi.NewStorageContainersApi(newClusterMgmtClient(c.cfg))

	names := make(map[string]string)
	page := 0
	limit := 500

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		pageArg := page
		limitArg := limit
		resp, err := api.ListStorageContainers(&pageArg, &limitArg, nil, nil, nil, nil)
		if err != nil {
			return nil, fmt.Errorf("list storage containers page %d: %w", page, err)
		}
		if resp == nil || resp.GetData() == nil {
			break
		}

		containers, ok := resp.GetData().([]clusterconfig.StorageContainer)
		if !ok {
			return nil, fmt.Errorf("unexpected storage container response type %T", resp.GetData())
		}

		for i := range containers {
			sc := &containers[i]
			name := derefString(sc.Name)
			if extID := derefString(sc.ExtId); extID != "" && name != "" {
				names[extID] = name
			}
			if containerExtID := derefString(sc.ContainerExtId); containerExtID != "" && name != "" {
				names[containerExtID] = name
			}
		}

		if len(containers) < limit {
			break
		}
		page++
	}

	return names, nil
}
