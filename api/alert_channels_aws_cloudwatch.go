//
// Author:: Vatasha White (<vatasha.white@lacework.net>)
// Copyright:: Copyright 2021, Lacework Inc.
// License:: Apache License, Version 2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package api

// GetAWSCloudwatch gets a single instance of an AWS Cloudwatch alert channel
// with the corresponding integration guid
func (svc *AlertChannelsService) GetAWSCloudwatch(guid string) (response AWSCloudwatchAlertChannelResponseV2, err error) {
	err = svc.get(guid, &response)
	return
}

// UpdateAWSCloudwatch Update AWSCloudWatch updates a single instance of an AWS cloudwatch integration on the Lacework server
func (svc *AlertChannelsService) UpdateAWSCloudwatch(data AlertChannel) (response AWSCloudwatchAlertChannelResponseV2, err error) {
	err = svc.update(data.ID(), data, &response)
	return
}

type AWSCloudwatchDataV2 struct {
	EventBusArn string `json:"eventBusArn"`
}

type AWSCloudwatchAlertChannelV2 struct {
	v2CommonIntegrationData
	Data AWSCloudwatchDataV2 `json:"data"`
}

type AWSCloudwatchAlertChannelResponseV2 struct {
	Data AWSCloudwatchAlertChannelV2 `json:"data"`
}
