package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
)

var (
	apiKey       string
	baseURL      string
	modelName    string
	inputFile    string
	outputFile   string
	outputFormat string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "subai",
		Short: "Subtitle translation agent powered by Eino",
		Long:  "A subtitle translation agent that translates English subtitles to Chinese-English bilingual subtitles using the Eino framework.",
		Run:   run,
	}

	rootCmd.Flags().StringVarP(&apiKey, "api-key", "k", "", "OpenAI API key (required)")
	rootCmd.Flags().StringVarP(&baseURL, "base-url", "u", "", "Custom base URL for OpenAI API")
	rootCmd.Flags().StringVarP(&modelName, "model", "m", "gpt-3.5-turbo", "Model name to use for translation")
	rootCmd.Flags().StringVarP(&inputFile, "input", "i", "", "Input subtitle file path (required)")
	rootCmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output subtitle file path (required)")
	rootCmd.Flags().StringVarP(&outputFormat, "format", "f", "srt", "Output format (srt or ass)")

	rootCmd.MarkFlagRequired("api-key")
	rootCmd.MarkFlagRequired("input")
	rootCmd.MarkFlagRequired("output")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Printf("[Main] SubAI 字幕翻译 Agent 启动")
	log.Printf("[Main] 配置 - 模型: %s, 输入: %s, 输出: %s, 格式: %s", modelName, inputFile, outputFile, outputFormat)

	ctx := context.Background()

	agent, err := NewSubtitleAgent(ctx, apiKey, baseURL, modelName)
	if err != nil {
		log.Printf("[Main] 创建 Agent 失败: %v", err)
		fmt.Fprintf(os.Stderr, "Failed to create agent: %v\n", err)
		os.Exit(1)
	}

	input := AgentInput{
		SubtitlePath: inputFile,
		OutputPath:   outputFile,
		OutputFormat: outputFormat,
	}

	output, err := agent.Run(ctx, input)
	if err != nil {
		log.Printf("[Main] 运行失败: %v", err)
		fmt.Fprintf(os.Stderr, "Failed to run agent: %v\n", err)
		os.Exit(1)
	}

	if output.Success {
		log.Printf("[Main] 完成: %s", output.Message)
		fmt.Println(output.Message)
	} else {
		log.Printf("[Main] 翻译失败: %s", output.Message)
		fmt.Fprintf(os.Stderr, "Translation failed: %s\n", output.Message)
		os.Exit(1)
	}
}
