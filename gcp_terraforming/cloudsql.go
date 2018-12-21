// Copyright 2018 The Terraformer Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gcp_terraforming

import (
	"context"
	"log"
	"net/http"
	"waze/terraformer/terraform_utils"

	"golang.org/x/oauth2/google"

	"cloud.google.com/go/monitoring/apiv3"
	"google.golang.org/api/iterator"
	"google.golang.org/api/sqladmin/v1beta4"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
)

var cloudSQLAllowEmptyValues = []string{}

var cloudSQLAdditionalFields = map[string]string{}

type CloudSQLGenerator struct {
	GCPService
}

func (g *CloudSQLGenerator) loadDBInstances(svc *sqladmin.Service, project string) error {
	dbInstances, err := svc.Instances.List(project).Do()
	if err != nil {
		return err
	}
	for _, dbInstance := range dbInstances.Items {
		g.Resources = append(g.Resources, terraform_utils.NewResource(
			dbInstance.Name,
			dbInstance.Name,
			"google_sql_database_instance",
			"google",
			map[string]string{},
			cloudSQLAllowEmptyValues,
			cloudSQLAdditionalFields,
		))
		err := g.loadDBs(svc, dbInstance.Name, project)
		if err != nil {
			return err
		}
	}

	return nil
}

func (g *CloudSQLGenerator) loadDBs(svc *sqladmin.Service, instanceName, project string) error {
	DBs, err := svc.Databases.List(project, instanceName).Do()
	if err != nil {
		return err
	}
	for _, db := range DBs.Items {
		g.Resources = append(g.Resources, terraform_utils.NewResource(
			instanceName+":"+db.Name,
			instanceName+"-"+db.Name,
			"google_sql_database",
			"google",
			map[string]string{},
			cloudSQLAllowEmptyValues,
			cloudSQLAdditionalFields,
		))
	}
	return nil
}

func (g *CloudSQLGenerator) loadSSLCert(ctx context.Context, project string) error {
	client, err := monitoring.NewNotificationChannelClient(ctx)
	if err != nil {
		return err
	}

	req := &monitoringpb.ListNotificationChannelsRequest{
		Name: "projects/" + project,
	}

	notificationChannelIterator := client.ListNotificationChannels(ctx, req)
	for {
		notificationChannel, err := notificationChannelIterator.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Println("error with notification Channel:", err)
			continue
		}
		g.Resources = append(g.Resources, terraform_utils.NewResource(
			notificationChannel.Name,
			notificationChannel.Name,
			"google_monitoring_notification_channel",
			"google",
			map[string]string{
				"name": notificationChannel.Name,
			},
			monitoringAllowEmptyValues,
			monitoringAdditionalFields,
		))
	}
	return nil
}

func (g *CloudSQLGenerator) loadSQLUser(ctx context.Context, project string) error {
	client, err := monitoring.NewUptimeCheckClient(ctx)
	if err != nil {
		return err
	}

	req := &monitoringpb.ListUptimeCheckConfigsRequest{
		Parent: "projects/" + project,
	}

	uptimeCheckConfigsIterator := client.ListUptimeCheckConfigs(ctx, req)
	for {
		uptimeCheckConfigs, err := uptimeCheckConfigsIterator.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Println("error with uptimeCheckConfigs:", err)
			continue
		}
		g.Resources = append(g.Resources, terraform_utils.NewResource(
			uptimeCheckConfigs.Name,
			uptimeCheckConfigs.Name,
			"google_monitoring_uptime_check_config",
			"google",
			map[string]string{
				"name": uptimeCheckConfigs.Name,
			},
			monitoringAllowEmptyValues,
			monitoringAdditionalFields,
		))
	}
	return nil
}

// Generate TerraformResources from GCP API,
// from each alert  create 1 TerraformResource
// Need alert name as ID for terraform resource
func (g *CloudSQLGenerator) InitResources() error {
	project := g.GetArgs()["project"]
	ctx := context.Background()
	var client *http.Client
	var err error
	client, err = google.DefaultClient(ctx, sqladmin.SqlserviceAdminScope)
	svc, err := sqladmin.New(client)
	if err != nil {
		return err
	}
	if err := g.loadDBInstances(svc, project); err != nil {
		return err
	}

	g.PopulateIgnoreKeys()
	return nil
}
