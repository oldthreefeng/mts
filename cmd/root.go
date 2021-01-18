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

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "mts",
	Short: "mts is jd sanp up tools",
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) { 
		Start()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		logger.Error(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.mts.yaml)")
	rootCmd.PersistentFlags().StringVar(&skuId, "skuId", "100012043978", "茅台商品ID")
	rootCmd.PersistentFlags().IntVar(&num, "num", 2, "商品数量")
	rootCmd.PersistentFlags().IntVar(&works, "works", 5, "并发数")
	rootCmd.PersistentFlags().StringVar(&start, "start", "09:59:59.500", "秒杀开始时间---不带日期")
	rootCmd.PersistentFlags().StringVar(&brwoserPath, "brwoserPath", "", "chrome浏览器执行路径，路径不能有空格")
	rootCmd.PersistentFlags().StringVar(&eid, "eid", EnvDefault("JD_EID",""), "如果不传入，可自动获取，对于无法获取的用户可手动传入参数")
	rootCmd.PersistentFlags().StringVar(&fp, "fp",  EnvDefault("JD_FP",""), "如果不传入，可自动获取，对于无法获取的用户可手动传入参数")
	rootCmd.PersistentFlags().StringVar(&payPwd, "payPwd", "", "支付密码 可不填")
	rootCmd.PersistentFlags().BoolVarP(&version, "version", "v", false, "版本号")
	rootCmd.PersistentFlags().BoolVar(&isFileLog, "log", false, "是否使用文件记录日志")
	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if version {
		VersionStr()
		os.Exit(0)
	}
	if isFileLog {
		logger.Cfg(6, "mts.log")
	} else {
		logger.Cfg(6, "")
	}
}

var (
	skuId       string
	num         int
	works       int
	start       string
	brwoserPath string
	eid         string
	fp          string
	payPwd      string
	isFileLog   bool
	version 	bool
)


func EnvDefault(key, defVal string) string {
	val, ex := os.LookupEnv(key)
	// fmt.Println(val)
	if !ex || val == "" {
		return defVal
	}
	return val
}

func Start() {
	var err error
	execPath := ""
	if brwoserPath != "" {
		execPath = brwoserPath
	}
	RE:
	jdSnap := internal.NewjdSnap(execPath, skuId, num, works)
	jdSnap.StartTime, err = utils.Hour2Unix(start)
	if err != nil {
		logger.Fatal("开始时间初始化失败", err)
	}

	jdSnap.PayPwd = payPwd
	if eid != "" {
		if fp == "" {
			logger.Fatal("请传入fp参数")
		}
		jdSnap.SetEid(eid)
	}

	if fp != "" {
		if eid == "" {
			logger.Fatal("请传入eid参数")
		}
		jdSnap.SetFp(fp)
	}

	if jdSnap.StartTime.Unix() < time.Now().Unix() {
		jdSnap.StartTime = jdSnap.StartTime.AddDate(0, 0, 1)
	}
	jdSnap.SyncJdTime()
	logger.Info("开始执行时间为：", jdSnap.StartTime.Format(utils.DateTimeFormatStr))

	err = jdSnap.Run()
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