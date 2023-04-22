package jenkins

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type JenkinsBuilder struct {
	baseUrl  string
	username string
	token    string
}

func NewBuilder(baseUrl, username, token string) JenkinsBuilder {
	return JenkinsBuilder{
		baseUrl:  baseUrl,
		username: username,
		token:    token,
	}
}

func (j *JenkinsBuilder) RunBuild(jobName string, params map[string]string, callback func(int /* 1队列中, 2构建任务开始, 3构建任务结束/取消 */, string /* queueId/taskId/"" */, string /* taskUrl/result */, map[string]interface{}) /* Jenkins返回的数据 */) error {
	failedCount := 0

	p, err := json.Marshal(params)
	if err != nil {
		return err
	}

	log.Println("开始构建:", jobName, ", params:", string(p))

	failedCount = 0
	queueId := 0
	for {
		var err error
		queueId, err = j.startTask(jobName, params)
		if err == nil {
			break
		}
		if failedCount >= 10 {
			return err
		}
		failedCount = failedCount + 1
		time.Sleep(time.Second)
	}

	callback(1, fmt.Sprint(queueId), "", nil)

	log.Println("查询排队状态:", queueId)
	failedCount = 0
	taskId := ""
	for {
		res, err := j.getQueueInfo(strconv.Itoa(queueId))
		if err != nil {
			if failedCount >= 10 {
				return err
			}
			failedCount = failedCount + 1
			time.Sleep(time.Second)
			continue
		}
		if cancelled, ok := res["cancelled"]; ok && fmt.Sprint(cancelled) == "true" { // 正在排队的时候取消了
			callback(3, "", "ABORTED", nil)
			return nil
		}
		if executable, ok := res["executable"]; ok && executable != nil {
			temp := executable.(map[string]interface{})
			taskId = strconv.FormatFloat(temp["number"].(float64), 'f', -1, 32)
			break
		}
		time.Sleep(time.Second * 2)
	}

	log.Println("查询构建任务信息:", taskId)
	failedCount = 0
	statusNotified := false
	for {
		res, err := j.getTaskInfo(jobName, string(taskId))
		if err != nil {
			if failedCount >= 10 {
				return err
			}
			failedCount = failedCount + 1
			time.Sleep(time.Second)
			continue
		}
		if !statusNotified {
			statusNotified = true

			callback(2, fmt.Sprint(res["number"]), fmt.Sprint(res["url"]), res)
		}
		if fmt.Sprint(res["building"]) == "false" {
			callback(3, fmt.Sprint(res["number"]), fmt.Sprint(res["result"]), res) // SUCCESS, FAILURE, ABORTED
			break
		}
		time.Sleep(time.Second * 5)
	}
	return nil
}

// 发起构建请求
func (j *JenkinsBuilder) startTask(jobName string, params map[string]string) (int, error) {
	buildUrl := j.baseUrl + "job/" + jobName + "/build"
	if len(params) > 0 {
		query := url.Values{}
		for key, value := range params {
			query.Add(key, fmt.Sprintf("%v", value))
		}
		buildUrl = j.baseUrl + "job/" + jobName + "/buildWithParameters?" + query.Encode()
	}
	req, err := http.NewRequest(http.MethodPost, buildUrl, nil)
	if err != nil {
		fmt.Println(err)
		return -1, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(j.username, j.token)
	var client http.Client
	resp, err := client.Do(req)
	if err != nil {
		return -1, err
	}
	defer func(body io.ReadCloser) {
		_ = body.Close()
	}(resp.Body)
	status := resp.StatusCode
	if status != http.StatusCreated {
		return -1, errors.New("任务启动失败: " + strconv.Itoa(status))
	}
	location := resp.Header.Get("Location")
	// fmt.Println(location)
	splitUrl := strings.Split(location, "/")
	return strconv.Atoi(splitUrl[len(splitUrl)-2])
}

// 查询排队信息
func (j *JenkinsBuilder) getQueueInfo(queueId string) (map[string]interface{}, error) {
	queryUrl := j.baseUrl + "queue/item/" + queueId + "/api/json"

	req, err := http.NewRequest(http.MethodPost, queryUrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(j.username, j.token)

	var client http.Client
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func(body io.ReadCloser) {
		_ = body.Close()
	}(resp.Body)
	status := resp.StatusCode
	if status != http.StatusOK {
		return nil, errors.New("查询队列信息失败: " + strconv.Itoa(status))
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	// log.Println(string(b))
	var result map[string]interface{}
	if err := json.Unmarshal(b, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// 查询构建信息
func (j *JenkinsBuilder) getTaskInfo(jobName, taskId string) (map[string]interface{}, error) {
	queryUrl := j.baseUrl + "job/" + jobName + "/" + taskId + "/api/json"

	req, err := http.NewRequest(http.MethodGet, queryUrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(j.username, j.token)
	// 发送请求
	var client http.Client
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func(body io.ReadCloser) {
		_ = body.Close()
	}(resp.Body)
	status := resp.StatusCode
	if status != http.StatusOK {
		return nil, errors.New("查询任务信息失败: " + strconv.Itoa(status))
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	// log.Println(string(b))
	var result map[string]interface{}
	if err := json.Unmarshal(b, &result); err != nil {
		return nil, err
	}
	return result, nil
}
