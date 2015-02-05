package vizzini_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"

	. "github.com/onsi/gomega"

	"github.com/cloudfoundry-incubator/receptor"
	"github.com/cloudfoundry-incubator/route-emitter/cfroutes"
	"github.com/cloudfoundry-incubator/runtime-schema/models"
)

//Tasks

func TaskGetter(guid string) func() (receptor.TaskResponse, error) {
	return func() (receptor.TaskResponse, error) {
		return client.GetTask(guid)
	}
}

func TasksByDomainGetter(domain string) func() ([]receptor.TaskResponse, error) {
	return func() ([]receptor.TaskResponse, error) {
		return client.TasksByDomain(domain)
	}
}

func ClearOutTasksInDomain(domain string) {
	tasks, err := client.TasksByDomain(domain)
	Ω(err).ShouldNot(HaveOccurred())
	for _, task := range tasks {
		client.CancelTask(task.TaskGuid)
		client.DeleteTask(task.TaskGuid)
	}
	Eventually(TasksByDomainGetter(domain)).Should(BeEmpty())
}

func TaskWithGuid(guid string) receptor.TaskCreateRequest {
	return receptor.TaskCreateRequest{
		TaskGuid: guid,
		Domain:   domain,
		Action: &models.RunAction{
			Path: "bash",
			Args: []string{"-c", "echo 'some output' > /tmp/bar"},
		},
		Stack:      stack,
		MemoryMB:   128,
		DiskMB:     128,
		CPUWeight:  100,
		LogGuid:    guid,
		LogSource:  "VIZ",
		ResultFile: "/tmp/bar",
		Annotation: "arbitrary-data",
	}
}

//LRPs

func LRPGetter(guid string) func() (receptor.DesiredLRPResponse, error) {
	return func() (receptor.DesiredLRPResponse, error) {
		return client.GetDesiredLRP(guid)
	}
}

func ActualGetter(guid string, index int) func() (receptor.ActualLRPResponse, error) {
	return func() (receptor.ActualLRPResponse, error) {
		return client.ActualLRPByProcessGuidAndIndex(guid, index)
	}
}

func ActualByProcessGuidGetter(guid string) func() ([]receptor.ActualLRPResponse, error) {
	return func() ([]receptor.ActualLRPResponse, error) {
		return client.ActualLRPsByProcessGuid(guid)
	}
}

func ActualByDomainGetter(domain string) func() ([]receptor.ActualLRPResponse, error) {
	return func() ([]receptor.ActualLRPResponse, error) {
		return client.ActualLRPsByDomain(domain)
	}
}

func ClearOutDesiredLRPsInDomain(domain string) {
	lrps, err := client.DesiredLRPsByDomain(domain)
	Ω(err).ShouldNot(HaveOccurred())
	for _, lrp := range lrps {
		Ω(client.DeleteDesiredLRP(lrp.ProcessGuid)).Should(Succeed())
	}
	Eventually(ActualByDomainGetter(domain)).Should(BeEmpty())
}

func EndpointCurler(endpoint string) func() (int, error) {
	return func() (int, error) {
		resp, err := http.Get(endpoint)
		if err != nil {
			return 0, err
		}
		resp.Body.Close()
		return resp.StatusCode, nil
	}
}

func IndexCounter(guid string, optionalHttpClient ...*http.Client) func() (int, error) {
	return func() (int, error) {
		counts := map[int]bool{}
		for i := 0; i < 100; i++ {
			index, err := GetIndexFromEndpointFor(guid, optionalHttpClient...)
			if err != nil {
				return 0, err
			}
			if index == -1 {
				continue
			}
			counts[index] = true
		}
		return len(counts), nil
	}
}

func GetIndexFromEndpointFor(guid string, optionalHttpClient ...*http.Client) (int, error) {
	httpClient := http.DefaultClient
	if len(optionalHttpClient) == 1 {
		httpClient = optionalHttpClient[0]
	}
	url := "http://" + RouteForGuid(guid) + "/index"
	resp, err := httpClient.Get(url)
	if err != nil {
		return 0, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return -1, nil
	}
	content, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return 0, err
	}

	return strconv.Atoi(string(content))
}

func StartedAtGetter(guid string) func() (int64, error) {
	url := "http://" + RouteForGuid(guid) + "/started-at"
	return func() (int64, error) {
		resp, err := http.Get(url)
		if err != nil {
			return 0, err
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return 0, errors.New(fmt.Sprintf("invalid status code: %d", resp.StatusCode))
		}
		content, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return 0, err
		}
		return strconv.ParseInt(string(content), 10, 64)
	}
}

func RouteForGuid(guid string) string {
	return fmt.Sprintf("%s.%s", guid, routableDomainSuffix)
}

func DesiredLRPWithGuid(guid string) receptor.DesiredLRPCreateRequest {
	return receptor.DesiredLRPCreateRequest{
		ProcessGuid: guid,
		Domain:      domain,
		Instances:   1,
		Setup: &models.SerialAction{
			Actions: []models.Action{
				&models.DownloadAction{
					From:     "http://onsi-public.s3.amazonaws.com/grace.tar.gz",
					To:       ".",
					CacheKey: "grace",
				},
			},
		},
		Action: &models.RunAction{
			Path: "./grace",
			Env:  []models.EnvironmentVariable{{Name: "PORT", Value: "8080"}, {"ACTION_LEVEL", "COYOTE"}, {"OVERRIDE", "DAQUIRI"}},
		},
		Monitor: &models.RunAction{
			Path: "nc",
			Args: []string{"-z", "0.0.0.0", "8080"},
		},
		Stack:     stack,
		MemoryMB:  128,
		DiskMB:    128,
		CPUWeight: 100,
		Ports:     []uint16{8080},
		Routes: cfroutes.CFRoutes{
			{Port: 8080, Hostnames: []string{RouteForGuid(guid)}},
		}.RoutingInfo(),
		LogGuid:    guid,
		LogSource:  "VIZ",
		Annotation: "arbitrary-data",
	}
}
