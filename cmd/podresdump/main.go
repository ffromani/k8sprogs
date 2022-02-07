/*
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright 2020 Red Hat, Inc.
 */

package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"time"

	flag "github.com/spf13/pflag"

	podresourcesapi "k8s.io/kubelet/pkg/apis/podresources/v1"
	podresources "k8s.io/kubernetes/pkg/kubelet/apis/podresources"
	kubeletutil "k8s.io/kubernetes/pkg/kubelet/util"
)

const (
	defaultPodResourcesPath    = "/var/lib/kubelet/pod-resources"
	defaultNamespace           = "default"
	defaultPodResourcesTimeout = 10 * time.Second
	defaultPodResourcesMaxSize = 1024 * 1024 * 16 // 16 Mb
	// obtained these values from node e2e tests : https://github.com/kubernetes/kubernetes/blob/82baa26905c94398a0d19e1b1ecf54eb8acb6029/test/e2e_node/util.go#L70
)

type dumper struct {
	cli           podresourcesapi.PodResourcesListerClient
	autoReconnect bool
	namespace     string
	out           *log.Logger
}

func main() {
	var err error
	autoReconnect := flag.BoolP("autoreconnect", "A", false, "don't give up if connection fails.")
	podResourcesSocketPath := flag.StringP("socket", "S", defaultPodResourcesPath, "podresources socket path.")
	listNamespace := flag.StringP("listnamespace", "N", defaultNamespace, "namespace to check")
	endpoint := flag.StringP("endpoint", "E", "list", "list/watch/getallocatebleresources podresource API Endpoint")

	flag.Parse()

	sockPath, err := kubeletutil.LocalEndpoint(*podResourcesSocketPath, podresources.Socket)
	if err != nil {
		log.Fatalf("%s", err)
	}

	cli, conn, err := podresources.GetV1Client(sockPath, defaultPodResourcesTimeout, defaultPodResourcesMaxSize)
	if err != nil {
		log.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	dm := dumper{
		cli:           cli,
		autoReconnect: *autoReconnect,
		namespace:     *listNamespace,
		out:           log.New(os.Stdout, "", log.Ldate|log.Lmicroseconds),
	}

	switch *endpoint {
	case "list":
		Listing(dm)
	case "getallocatableresources":
		GetAllocatableResources(dm)
	// TODO: Add watch support
	default:
		log.Fatalf("unsupported endpoint: %q", *endpoint)
	}
}

func Listing(dm dumper) {
	resp, err := dm.cli.List(context.TODO(), &podresourcesapi.ListPodResourcesRequest{})
	for {
		if err == nil {
			break
		} else {
			if !dm.autoReconnect {
				log.Fatalf("failed to list: %v", err)
			} else {
				log.Printf("Can't receive response: %v.Get(_) = _, %v", dm.cli, err)
				time.Sleep(1 * time.Second)
			}
		}
	}

	for _, podResource := range resp.GetPodResources() {
		if podResource.GetNamespace() != dm.namespace {
			log.Printf("SKIP pod %q\n", podResource.Name)
			continue
		}

		jsonBytes, err := json.Marshal(podResource)
		if err != nil {
			log.Printf("%v", err)
		} else {
			dm.out.Printf("%s\n", string(jsonBytes))
		}
	}
}

func GetAllocatableResources(dm dumper) {
	resp, err := dm.cli.GetAllocatableResources(context.TODO(), &podresourcesapi.AllocatableResourcesRequest{})
	for {
		if err == nil {
			break
		} else {
			if !dm.autoReconnect {
				log.Fatalf("failed to show GetAllocatableResources: %v", err)
			} else {
				log.Printf("Can't receive response: %v.Get(_) = _, %v", dm.cli, err)
				time.Sleep(1 * time.Second)
			}
		}
	}

	jsonBytes, err := json.Marshal(resp)
	if err != nil {
		log.Printf("%v", err)
	} else {
		dm.out.Printf("%s\n", string(jsonBytes))
	}

}
