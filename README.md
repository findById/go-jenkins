# 使用Go触发Jenkins构建

## 导入项目
```shell
go get -u github.com/findbyid/go-jenkins@latest
```

## 使用Docker启动一个Jenkins
```shell
sudo docker pull jenkins/jenkins:lts-jdk11
sudo docker run -itd -p 8082:8080 -v /etc/localtime:/etc/localtime --restart=always --name jenkins jenkins/jenkins:lts-jdk11
```

## 简单使用
```go
package main

import (
	"github.com/findbyid/go-jenkins/jenkins"
	"fmt"
)

func main() {

	jobName := "jenkins-job-name"

	params := make(map[string]string, 0)
	params["GIT_URL"] = ""
	params["BRANCH"] = "master"

	b := jenkins.NewBuilder("http://localhost:8082/", "admin", "admin")
	err := b.RunBuild(jobName, params, func(status int, number, data string, extra map[string]interface{}) {
		if status == 1 { // 构建队列中
			fmt.Println("=====queueId:", number)
		} else if status == 2 { // 构建任务开始
			fmt.Println("=====taskId:", number, " url:", data)
		} else if status == 3 { // 构建任务结束
			fmt.Println("=====taskId:", number, " result:", data) // SUCCESS, FAILURE, ABORTED
		}
	})
	if err != nil {
		fmt.Println("=====失败了")
		panic(err)
	}
}
```
