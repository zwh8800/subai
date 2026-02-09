# SubAI - 字幕翻译 Agent

基于 cloudwego/eino 框架开发的大模型字幕翻译 Agent，支持将英文电影字幕文件（SRT、ASS 格式）翻译成中英双语字幕。

## 功能特性

- **多格式支持**：支持 SRT 和 ASS 格式的字幕文件
- **智能背景分析**：翻译前自动分析字幕内容和文件名，总结电影/电视剧的背景信息，提高翻译准确性
- **分组翻译**：将时间相近的字幕（3秒内）分组翻译，提供更好的上下文
- **严格行数匹配**：使用 JSON 数组格式确保输入输出行数严格匹配
- **双语字幕输出**：生成中英双语字幕文件
- **ASS 样式优化**：中文字幕使用较大白色字体（20号），英文字幕使用较小牛皮纸色字体（16号）
- **Eino 框架集成**：使用 Eino 框架的 ChatModel 组件和 Chain 编排
- **astisub 库支持**：统一使用 astisub 库进行字幕解析和生成

## 安装

```bash
go build -o subai
```

## 使用方法

```bash
./subai -k <api-key> -i <input-file> -o <output-file> [options]
```

### 参数说明

- `-k, --api-key`: OpenAI API Key（必需）
- `-u, --base-url`: 自定义 OpenAI API Base URL（可选）
- `-m, --model`: 使用的模型名称（默认：gpt-3.5-turbo）
- `-i, --input`: 输入字幕文件路径（必需）
- `-o, --output`: 输出字幕文件路径（必需）
- `-f, --format`: 输出格式，srt 或 ass（默认：srt）

### 示例

翻译 SRT 文件：

```bash
./subai -k sk-xxx -i input.srt -o output.srt
```

翻译 ASS 文件（带样式优化）：

```bash
./subai -k sk-xxx -i input.ass -o output.ass -f ass
```

使用阿里云通义千问 API：

```bash
./subai -k sk-xxx -u https://dashscope.aliyuncs.com/compatible-mode/v1 -m qwen-plus -i input.srt -o output.srt
```

## 翻译流程

1. **解析字幕**：使用 astisub 库解析输入字幕文件
2. **背景分析**：分析字幕内容（前2000字符）和文件名，总结电影背景信息
3. **分组处理**：根据时间间隔（3秒）将字幕分组，提供更好的翻译上下文
4. **智能翻译**：结合背景信息和分组上下文进行翻译，确保行数严格匹配
5. **生成输出**：使用 astisub 库生成双语字幕文件

## 技术细节

### 背景信息总结
- 自动提取字幕前20000字符作为样本
- 结合文件名分析电影类型、主题和背景
- 生成的背景信息作为系统提示词的一部分

### 字幕分组算法
- 默认时间间隔阈值：3秒
- 将时间相近的字幕合并为一组
- 同组字幕一起翻译，保持上下文连贯性

### 翻译格式保证
- 使用 JSON 数组格式传输字幕内容
- 要求 AI 严格保持数组元素数量不变
- 自动清理 markdown 格式，确保 JSON 解析成功

### ASS 格式样式
- **中文样式**：Arial 字体，20号大小，白色 (&H00FFFFFF)
- **英文样式**：Arial 字体，16号大小，牛皮纸色 (&H00D2B48C)
- 双行显示：中文在上，英文在下

## 项目结构

- `main.go`: 主程序入口和命令行参数处理（基于 cobra）
- `agent.go`: 基于 Eino Chain 的字幕翻译 Agent，编排整个翻译流程
- `translator.go`: 翻译器实现，包含背景信息总结、字幕分组和翻译功能
- `subtitle.go`: 字幕文件解析和生成（基于 astisub 库）

## 依赖

- github.com/cloudwego/eino: Eino 框架
- github.com/cloudwego/eino-ext: Eino 扩展组件
- github.com/asticode/go-astisub: 字幕解析库
- github.com/spf13/cobra: 命令行参数处理

## License

MIT