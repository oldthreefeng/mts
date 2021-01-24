/*
Copyright © 2021 NAME HERE <EMAIL ADDRESS>

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
package cmd

import (
	"bufio"
	"os"
	"strings"
	"time"

	"github.com/oldthreefeng/mts/internal"
	"github.com/oldthreefeng/mts/pkg/logger"
	"github.com/oldthreefeng/mts/pkg/utils"
	"github.com/spf13/cobra"
)

// tmCmd represents the tm command
var tmCmd = &cobra.Command{
	Use:   "tm",
	Short: "tm 秒杀",
	Run: func(cmd *cobra.Command, args []string) {
		tm()
	},
}

func init() {
	rootCmd.AddCommand(tmCmd)
}

func tm() {
	var err error
	execPath := ""
	if start == ""{
		start = "19:59:58"
	}
	if skuId == "" {
		skuId = "20739895092"
	}
	RE:
	tmSecKill := internal.NewTmSecKill(execPath, skuId, num, works)
	tmSecKill.StartTime, err = utils.Hour2Unix(start)
	if tmSecKill.StartTime.Unix() < time.Now().Unix() {
		tmSecKill.StartTime = tmSecKill.StartTime.AddDate(0, 0, 1)
	}
	logger.Info("开始执行时间为：", tmSecKill.StartTime.Format(utils.DateTimeFormatStr))

	err = tmSecKill.Run()
	if err != nil {
		if strings.Contains(err.Error(), "exec") {
			logger.Info("默认浏览器执行路径未找到，"+execPath+"  请重新输入：")
			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				execPath = scanner.Text()
				if execPath != "" {
					break
				}
			}
			goto RE
		}
		logger.Fatal(err)
	}
} 	