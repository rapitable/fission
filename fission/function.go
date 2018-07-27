/*
Copyright 2016 The Fission Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/urfave/cli"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/fission/fission"
	"github.com/fission/fission/fission/log"
	"github.com/fission/fission/fission/logdb"
	"github.com/fission/fission/fission/portforward"
	"github.com/fission/fission/fission/sdk"
)

func printPodLogs(c *cli.Context) error {
	fnName := c.String("name")
	if len(fnName) == 0 {
		return sdk.MissingArgError("name")
	}

	queryURL, err := url.Parse(sdk.GetServerUrl())
	if err != nil {
		return sdk.FailedToError(err, "parse the base URL")
	}
	queryURL.Path = fmt.Sprintf("/proxy/logs/%s", fnName)

	req, err := http.NewRequest("POST", queryURL.String(), nil)
	if err != nil {
		return sdk.FailedToError(err, "create logs request")
	}

	httpClient := http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		return sdk.FailedToError(err, "execute get logs request")
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.New("get logs from pod directly")
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return sdk.FailedToError(err, "read the response body")
	}
	fmt.Println(string(body))
	return nil
}

// From this change onwards, we mandate that a function should reference a secret, config map and package in its own ns
func fnCreate(c *cli.Context) error {

	fnName := c.String("name")
	spec := false
	if c.Bool("spec") {
		spec = true
	}
	entrypoint := c.String("entrypoint")
	pkgName := c.String("pkg")
	secretName := c.String("secret")
	cfgMapName := c.String("configmap")
	envName := c.String("env")
	srcArchiveName := c.String("src")
	deployArchiveName := c.String("code")
	if len(deployArchiveName) == 0 {
		deployArchiveName = c.String("deploy")
	}
	buildcmd := c.String("buildcmd")
	minscale := c.Int("minscale")
	maxscale := c.Int("maxscale")
	executortype := c.String("executortype")
	mincpu := c.Int("mincpu")
	maxcpu := c.Int("maxcpu")
	minmemory := c.Int("minmemory")
	maxmemory := c.Int("maxmemory")
	targetCPU := c.Int("targetcpu")
	triggerURL := c.String("url")
	method := c.String("method")
	client := sdk.GetClient(c.GlobalString("server"))
	fnNamespace := c.String("fnNamespace")
	envNamespace := c.String("envNamespace")

	createFunctionArg := &sdk.CreateFunctionArg{
		FnName:            fnName,
		Spec:              spec,
		EntryPoint:        entrypoint,
		PkgName:           pkgName,
		SecretName:        secretName,
		CfgMapName:        cfgMapName,
		EnvName:           envName,
		SrcArchiveName:    srcArchiveName,
		DeployArchiveName: deployArchiveName,
		BuildCommand:      buildcmd,
		TriggerURL:        triggerURL,
		Method:            method,
		MinScale:          minscale,
		MaxScale:          maxscale,
		ExecutorType:      executortype,
		MinCPU:            mincpu,
		MaxCPU:            maxcpu,
		MinMemory:         minmemory,
		MaxMemory:         maxmemory,
		TargetCPU:         targetCPU,
		Client:            client,
		FnNamespace:       fnNamespace,
		EnvNamespace:      envNamespace,
	}

	err := sdk.CreateFunction(createFunctionArg)
	if err != nil {
		return sdk.FailedToError(err, "create function")
	}
	return err

}

func fnGet(c *cli.Context) error {
	client := sdk.GetClient(c.GlobalString("server"))

	fnName := c.String("name")
	if len(fnName) == 0 {
		return sdk.MissingArgError("name")
	}
	fnNamespace := c.String("fnNamespace")
	m := &metav1.ObjectMeta{
		Name:      fnName,
		Namespace: fnNamespace,
	}
	fn, err := client.FunctionGet(m)
	if err != nil {
		return sdk.FailedToError(err, "get function")
	}

	pkg, err := client.PackageGet(&metav1.ObjectMeta{
		Name:      fn.Spec.Package.PackageRef.Name,
		Namespace: fn.Spec.Package.PackageRef.Namespace,
	})
	if err != nil {
		return sdk.FailedToError(err, "get package")
	}

	os.Stdout.Write(pkg.Spec.Deployment.Literal)
	return err
}

func fnGetMeta(c *cli.Context) error {
	client := sdk.GetClient(c.GlobalString("server"))

	fnName := c.String("name")
	if len(fnName) == 0 {
		return sdk.MissingArgError("name")
	}
	fnNamespace := c.String("fnNamespace")

	m := &metav1.ObjectMeta{
		Name:      fnName,
		Namespace: fnNamespace,
	}

	f, err := client.FunctionGet(m)
	if err != nil {
		return sdk.FailedToError(err, "get function")
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)
	fmt.Fprintf(w, "%v\t%v\t%v\n", "NAME", "UID", "ENV")
	fmt.Fprintf(w, "%v\t%v\t%v\n",
		f.Metadata.Name, f.Metadata.UID, f.Spec.Environment.Name)
	w.Flush()
	return err
}

func fnUpdate(c *cli.Context) error {
	client := sdk.GetClient(c.GlobalString("server"))

	if len(c.String("package")) > 0 {
		return sdk.GeneralError("--package is deprecated, please use --deploy instead.")
	}

	if len(c.String("srcpkg")) > 0 {
		return sdk.GeneralError("--srcpkg is deprecated, please use --src instead.")
	}

	fnName := c.String("name")
	if len(fnName) == 0 {
		return sdk.MissingArgError("name")
	}
	fnNamespace := c.String("fnNamespace")

	function, err := client.FunctionGet(&metav1.ObjectMeta{
		Name:      fnName,
		Namespace: fnNamespace,
	})
	if err != nil {
		return sdk.FailedToError(err, fmt.Sprintf("read function '%v'", fnName))
	}

	envName := c.String("env")
	envNamespace := c.String("envNamespace")
	deployArchiveName := c.String("code")
	if len(deployArchiveName) == 0 {
		deployArchiveName = c.String("deploy")
	}
	srcArchiveName := c.String("src")
	pkgName := c.String("pkg")
	entrypoint := c.String("entrypoint")
	buildcmd := c.String("buildcmd")
	force := c.Bool("force")

	secretName := c.String("secret")
	cfgMapName := c.String("configmap")

	if len(srcArchiveName) > 0 && len(deployArchiveName) > 0 {
		return sdk.MissingArgError("Need either of --src or --deploy and not both arguments.")
	}

	if len(secretName) > 0 {
		if len(function.Spec.Secrets) > 1 {
			return sdk.GeneralError("Please use 'fission spec apply' to update list of secrets")
		}

		// check that the referenced secret is in the same ns as the function, if not give a warning.
		_, err := client.SecretGet(&metav1.ObjectMeta{
			Namespace: fnNamespace,
			Name:      secretName,
		})
		if k8serrors.IsNotFound(err) {
			log.Warn(fmt.Sprintf("secret %s not found in Namespace: %s. Secret needs to be present in the same namespace as function", secretName, fnNamespace))
		}

		newSecret := fission.SecretReference{
			Name:      secretName,
			Namespace: fnNamespace,
		}
		function.Spec.Secrets = []fission.SecretReference{newSecret}
	}

	if len(cfgMapName) > 0 {
		if len(function.Spec.ConfigMaps) > 1 {
			return sdk.GeneralError("Please use 'fission spec apply' to update list of configmaps")
		}

		// check that the referenced cfgmap is in the same ns as the function, if not give a warning.
		_, err := client.ConfigMapGet(&metav1.ObjectMeta{
			Namespace: fnNamespace,
			Name:      cfgMapName,
		})
		if k8serrors.IsNotFound(err) {
			log.Warn(fmt.Sprintf("ConfigMap %s not found in Namespace: %s. ConfigMap needs to be present in the same namespace as the function", cfgMapName, fnNamespace))
		}

		newCfgMap := fission.ConfigMapReference{
			Name:      cfgMapName,
			Namespace: fnNamespace,
		}
		function.Spec.ConfigMaps = []fission.ConfigMapReference{newCfgMap}
	}

	if len(envName) > 0 {
		function.Spec.Environment.Name = envName
		function.Spec.Environment.Namespace = envNamespace
	}

	if len(entrypoint) > 0 {
		function.Spec.Package.FunctionName = entrypoint
	}
	if len(pkgName) == 0 {
		pkgName = function.Spec.Package.PackageRef.Name
	}

	pkg, err := client.PackageGet(&metav1.ObjectMeta{
		Namespace: fnNamespace,
		Name:      pkgName,
	})
	if err != nil {
		return sdk.FailedToError(err, fmt.Sprintf("read package '%v.%v'. Pkg should be present in the same ns as the function", pkgName, fnNamespace))
	}

	pkgMetadata := &pkg.Metadata

	if len(deployArchiveName) != 0 || len(srcArchiveName) != 0 || len(buildcmd) != 0 || len(envName) != 0 {
		fnList, err := sdk.GetFunctionsByPackage(client, pkg.Metadata.Name, pkg.Metadata.Namespace)
		if err != nil {
			return sdk.FailedToError(err, "get function list")
		}

		if !force && len(fnList) > 1 {
			return sdk.GeneralError("Package is used by multiple functions, use --force to force update")
		}

		pkgMetadata, err = sdk.UpdatePackage(client, pkg, envName, envNamespace, srcArchiveName, deployArchiveName, buildcmd)
		if err != nil {
			return sdk.FailedToError(err, fmt.Sprintf("update package '%v'", pkgName))
		}

		fmt.Printf("package '%v' updated\n", pkgMetadata.GetName())

		// update resource version of package reference of functions that shared the same package
		for _, fn := range fnList {
			// ignore the update for current function here, it will be updated later.
			if fn.Metadata.Name != fnName {
				fn.Spec.Package.PackageRef.ResourceVersion = pkgMetadata.ResourceVersion
				_, err := client.FunctionUpdate(&fn)
				if err != nil {
					return sdk.FailedToError(err, "update function")
				}
			}
		}
	}

	// TODO : One corner case where user just updates the pkg reference with fnUpdate, but internally this new pkg reference
	// references a diff env than the spec

	// update function spec with new package metadata
	function.Spec.Package.PackageRef = fission.PackageRef{
		Namespace:       pkgMetadata.Namespace,
		Name:            pkgMetadata.Name,
		ResourceVersion: pkgMetadata.ResourceVersion,
	}

	function.Spec.Resources, err = getResourceReq(c, function.Spec.Resources)
	if err != nil {
		return sdk.FailedToError(err, "get resource requirements")
	}

	if c.IsSet("targetcpu") {

		function.Spec.InvokeStrategy.ExecutionStrategy.TargetCPUPercent, err = sdk.GetTargetCPU(c.Int("targetcpu"))
		if err != nil {
			return err
		}
	}

	if c.IsSet("minscale") {
		minscale := c.Int("minscale")
		maxscale := c.Int("maxscale")
		if c.IsSet("maxscale") && minscale > c.Int("maxscale") {
			return sdk.GeneralError(fmt.Sprintf("Minscale's value %v can not be greater than maxscale value %v", minscale, maxscale))
		}
		if function.Spec.InvokeStrategy.ExecutionStrategy.ExecutorType != fission.ExecutorTypePoolmgr &&
			minscale > function.Spec.InvokeStrategy.ExecutionStrategy.MaxScale {
			return sdk.GeneralError(fmt.Sprintf("Minscale provided: %v can not be greater than maxscale of existing function: %v", minscale,
				function.Spec.InvokeStrategy.ExecutionStrategy.MaxScale))
		}
		function.Spec.InvokeStrategy.ExecutionStrategy.MinScale = minscale
	}

	if c.IsSet("maxscale") {
		maxscale := c.Int("maxscale")
		if maxscale < function.Spec.InvokeStrategy.ExecutionStrategy.MinScale {
			return sdk.GeneralError(fmt.Sprintf("Function's minscale: %v can not be greater than maxscale provided: %v",
				function.Spec.InvokeStrategy.ExecutionStrategy.MinScale, maxscale))
		}
		function.Spec.InvokeStrategy.ExecutionStrategy.MaxScale = maxscale
	}

	if c.IsSet("executortype") {
		var fnExecutor fission.ExecutorType
		switch c.String("executortype") {
		case "":
			fnExecutor = fission.ExecutorTypePoolmgr
		case fission.ExecutorTypePoolmgr:
			fnExecutor = fission.ExecutorTypePoolmgr
		case fission.ExecutorTypeNewdeploy:
			fnExecutor = fission.ExecutorTypeNewdeploy
		default:
			return sdk.GeneralError("Executor type must be one of 'poolmgr' or 'newdeploy', defaults to 'poolmgr'")
		}
		if (c.IsSet("mincpu") || c.IsSet("maxcpu") || c.IsSet("minmemory") || c.IsSet("maxmemory")) &&
			fnExecutor == fission.ExecutorTypePoolmgr {
			log.Warn("CPU/Memory specified for function with pool manager executor will be ignored in favor of resources specified at environment")
		}
		function.Spec.InvokeStrategy.ExecutionStrategy.ExecutorType = fnExecutor
	}

	_, err = client.FunctionUpdate(function)
	if err != nil {
		return sdk.FailedToError(err, "update function")
	}

	fmt.Printf("function '%v' updated\n", fnName)
	return err
}

func fnDelete(c *cli.Context) error {
	client := sdk.GetClient(c.GlobalString("server"))

	fnName := c.String("name")
	if len(fnName) == 0 {
		return sdk.MissingArgError("name")
	}
	fnNamespace := c.String("fnNamespace")

	m := &metav1.ObjectMeta{
		Name:      fnName,
		Namespace: fnNamespace,
	}

	err := client.FunctionDelete(m)
	if err != nil {
		return sdk.FailedToError(err, fmt.Sprintf("delete function '%v'", fnName))
	}

	fmt.Printf("function '%v' deleted\n", fnName)
	return err
}

func fnList(c *cli.Context) error {
	client := sdk.GetClient(c.GlobalString("server"))
	ns := c.String("fnNamespace")

	fns, err := client.FunctionList(ns)
	if err != nil {
		return sdk.FailedToError(err, "list functions")
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)

	fmt.Fprintf(w, "%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\n", "NAME", "UID", "ENV", "EXECUTORTYPE", "MINSCALE", "MAXSCALE", "MINCPU", "MAXCPU", "MINMEMORY", "MAXMEMORY", "TARGETCPU")
	for _, f := range fns {
		mincpu := f.Spec.Resources.Requests.Cpu
		mincpu().Value()
		fmt.Fprintf(w, "%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\n",
			f.Metadata.Name, f.Metadata.UID, f.Spec.Environment.Name,
			f.Spec.InvokeStrategy.ExecutionStrategy.ExecutorType,
			f.Spec.InvokeStrategy.ExecutionStrategy.MinScale,
			f.Spec.InvokeStrategy.ExecutionStrategy.MaxScale,
			f.Spec.Resources.Requests.Cpu().String(),
			f.Spec.Resources.Limits.Cpu().String(),
			f.Spec.Resources.Requests.Memory().String(),
			f.Spec.Resources.Limits.Memory().String(),
			f.Spec.InvokeStrategy.ExecutionStrategy.TargetCPUPercent)
	}
	w.Flush()

	return err
}

func fnLogs(c *cli.Context) error {

	client := sdk.GetClient(c.GlobalString("server"))

	fnName := c.String("name")
	if len(fnName) == 0 {
		return sdk.MissingArgError("name")
	}
	fnNamespace := c.String("fnNamespace")

	dbType := c.String("dbtype")
	if len(dbType) == 0 {
		dbType = logdb.INFLUXDB
	}

	fnPod := c.String("pod")
	m := &metav1.ObjectMeta{
		Name:      fnName,
		Namespace: fnNamespace,
	}

	recordLimit := c.Int("recordcount")
	if recordLimit <= 0 {
		recordLimit = 1000
	}

	f, err := client.FunctionGet(m)
	if err != nil {
		return sdk.FailedToError(err, "get function")
	}

	// request the controller to establish a proxy server to the database.
	logDB, err := logdb.GetLogDB(dbType, sdk.GetServerUrl())
	if err != nil {
		return sdk.GeneralError("failed to connect log database")
	}

	requestChan := make(chan struct{})
	responseChan := make(chan struct{})
	ctx := context.Background()

	go func(ctx context.Context, requestChan, responseChan chan struct{}) {
		t := time.Unix(0, 0*int64(time.Millisecond))
		for {
			select {
			case <-requestChan:
				logFilter := logdb.LogFilter{
					Pod:         fnPod,
					Function:    f.Metadata.Name,
					FuncUid:     string(f.Metadata.UID),
					Since:       t,
					RecordLimit: recordLimit,
				}
				logEntries, err := logDB.GetLogs(logFilter)
				if err != nil {
					//TODO refactor to use error channel (at latest when moving to sdk package)
					logErrorAndExit("failed to query logs")
				}
				for _, logEntry := range logEntries {
					if c.Bool("d") {
						fmt.Printf("Timestamp: %s\nNamespace: %s\nFunction Name: %s\nFunction ID: %s\nPod: %s\nContainer: %s\nStream: %s\nLog: %s\n---\n",
							logEntry.Timestamp, logEntry.Namespace, logEntry.FuncName, logEntry.FuncUid, logEntry.Pod, logEntry.Container, logEntry.Stream, logEntry.Message)
					} else {
						fmt.Printf("[%s] %s\n", logEntry.Timestamp, logEntry.Message)
					}
					t = logEntry.Timestamp
				}
				responseChan <- struct{}{}
			case <-ctx.Done():
				return
			}
		}
	}(ctx, requestChan, responseChan)

	for {
		requestChan <- struct{}{}
		<-responseChan
		if !c.Bool("f") {
			ctx.Done()
			return nil
		}
		time.Sleep(1 * time.Second)
	}
}

func fnTest(c *cli.Context) error {
	fnName := c.String("name")
	if len(fnName) == 0 {
		return sdk.MissingArgError("name")
	}
	ns := c.String("fnNamespace")

	routerURL := os.Getenv("FISSION_ROUTER")
	if len(routerURL) == 0 {
		// Portforward to the fission router
		localRouterPort := portforward.Setup(sdk.GetKubeConfigPath(),
			sdk.GetFissionNamespace(), "application=fission-router")
		routerURL = "127.0.0.1:" + localRouterPort
	} else {
		routerURL = strings.TrimPrefix(routerURL, "http://")
	}

	fnUri := fnName
	if ns != metav1.NamespaceDefault {
		fnUri = fmt.Sprintf("%v/%v", ns, fnName)
	}

	url := fmt.Sprintf("http://%s/fission-function/%s", routerURL, fnUri)

	resp := sdk.HttpRequest(c.String("method"), url, c.String("body"), c.StringSlice("header"))
	if resp.StatusCode < 400 {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return sdk.FailedToError(err, "Function test")
		}
		fmt.Print(string(body))
		defer resp.Body.Close()
		return nil
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return sdk.FailedToError(err, "read log response from pod")
	}
	fmt.Printf("Error calling function %s: %d %s", fnName, resp.StatusCode, string(body))
	defer resp.Body.Close()
	err = printPodLogs(c)
	if err != nil {
		fnLogs(c)
	}

	return nil
}
