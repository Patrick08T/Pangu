package perforce

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Patrick08T/Pangu/internal/diagnostic"
	"io"
	"net"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port   string `json:"port"`
	User   string `json:"user"`
	Client string `json:"client"`
	Scope  string `json:"scope"`
}

type Record map[string]string

var identifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.@-]{0,127}$`)

func (c Config) Validate() error {
	host, port, err := net.SplitHostPort(strings.TrimPrefix(c.Port, "ssl:"))
	n, _ := strconv.Atoi(port)
	if err != nil || host == "" || strings.ContainsAny(host, "/\\ \r\n\t") || n < 1 || n > 65535 {
		return errors.New("Perforce 服务器需要 host:port 或 ssl:host:port")
	}
	if !identifier.MatchString(c.User) || !identifier.MatchString(c.Client) {
		return errors.New("Perforce 用户和模板 Client 标识无效")
	}
	if c.Scope == "" || path.IsAbs(c.Scope) || path.Clean(c.Scope) != c.Scope || strings.ContainsAny(c.Scope, "\\@#%*\r\n\t:") {
		return errors.New("项目子目录必须是相对路径；使用 . 表示映射根目录")
	}
	for _, part := range strings.Split(c.Scope, "/") {
		if part == ".." || strings.Contains(part, "...") || strings.EqualFold(part, ".git") {
			return errors.New("项目子目录不能越界或指向 Git 元数据")
		}
	}
	return nil
}

func (c Config) Files(client string) string {
	if c.Scope == "." {
		return "//" + client + "/..."
	}
	return "//" + client + "/" + c.Scope + "/..."
}

func Command(ctx context.Context, c Config, client, input string, args ...string) ([]Record, error) {
	return commandIn(ctx, c, client, input, "", args...)
}

func commandIn(ctx context.Context, c Config, client, input, directory string, args ...string) ([]Record, error) {
	info, err := commandInfo(ctx, c, client, directory)
	if err != nil {
		return nil, err
	}
	if len(args) == 1 && args[0] == "info" {
		return info, nil
	}
	charset, err := commandCharset(info, c.Scope, input, args)
	if err != nil {
		return nil, err
	}
	return runCommand(ctx, c, client, input, charset, directory, args...)
}

func commandInfo(ctx context.Context, c Config, client, directory string) ([]Record, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	if !identifier.MatchString(client) {
		return nil, errors.New("Perforce Client 标识无效")
	}
	return runCommand(ctx, c, client, "", "none", directory, "info")
}

func commandCharset(info []Record, scope, input string, args []string) (string, error) {
	charset := "none"
	if len(info) == 1 && info[0]["unicode"] == "enabled" {
		charset = "utf8"
	}
	if charset == "none" {
		for _, value := range append([]string{input, scope}, args...) {
			for _, r := range value {
				if r > 127 {
					return "", errors.New("非 Unicode Perforce 服务仅支持 ASCII 路径和表单；请使用 Unicode 服务处理中文文件名")
				}
			}
		}
	}
	return charset, nil
}

func commandProcess(ctx context.Context, c Config, client, input, charset, directory string, args ...string) (*exec.Cmd, error) {
	binary := os.Getenv("PANGU_P4_BINARY")
	if binary == "" {
		binary = "p4"
	}
	options := []string{"-C", charset, "-v", "net.maxwait=30", "-p", c.Port, "-u", c.User, "-c", client}
	if directory != "" {
		options = append(options, "-d", directory)
	}
	if charset == "utf8" {
		options = append(options, "-Q", "utf8")
	}
	if input == "" {
		options = append([]string{"-Mj", "-ztag"}, options...)
	}
	stdin, commandArgs, err := unicodeArguments(input, args)
	if err != nil {
		return nil, err
	}
	if stdin != input {
		options = append(options, "-x", "-")
	}
	cmd := exec.CommandContext(ctx, binary, append(options, commandArgs...)...)
	cmd.Dir = directory
	cmd.Env, cmd.Stdin, cmd.WaitDelay = environment(), strings.NewReader(stdin), time.Second
	return cmd, nil
}

func runCommand(ctx context.Context, c Config, client, input, charset, directory string, args ...string) ([]Record, error) {
	cmd, err := commandProcess(ctx, c, client, input, charset, directory, args...)
	if err != nil {
		return nil, err
	}
	out, stderr := &diagnostic.Buffer{Limit: 8 * 1024 * 1024}, &diagnostic.Buffer{Limit: 4096}
	cmd.Stdout, cmd.Stderr = out, stderr
	runErr := cmd.Run()
	if out.Truncated {
		return nil, errors.New("Perforce 返回超过 8MiB，请缩小项目范围")
	}
	records, err := commandOutput(out.Data.String(), input != "")
	if err != nil {
		return nil, err
	}
	if runErr != nil {
		for _, record := range records {
			if record["data"] != "" {
				_, _ = stderr.Write([]byte(record["data"]))
			}
		}
		message, _ := diagnostic.Text(stderr.Data.String(), 4096)
		return nil, fmt.Errorf("Perforce 操作失败: %w；%s", runErr, message)
	}
	return records, nil
}

// Windows 的 P4 argv 使用本机代码页；UTF-8 文件参数通过标准输入的参数表传递。
func unicodeArguments(input string, args []string) (string, []string, error) {
	command, files := []string{}, []string{}
	for _, arg := range args {
		unicode := false
		for _, r := range arg {
			unicode = unicode || r > 127
		}
		if !unicode {
			command = append(command, arg)
			continue
		}
		if input != "" || !strings.HasPrefix(arg, "//") || strings.ContainsAny(arg, "\r\n") {
			return "", nil, errors.New("Unicode 命令参数必须为受控文件路径")
		}
		files = append(files, arg)
	}
	if len(files) == 0 {
		return input, args, nil
	}
	return strings.Join(files, "\n") + "\n", command, nil
}

func commandOutput(output string, form bool) ([]Record, error) {
	if form {
		return []Record{{"data": output}}, nil
	}
	return parse(output)
}

func environment() []string {
	var result []string
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		switch strings.ToUpper(key) {
		case "P4CONFIG", "P4ENVIRO", "P4CLIENT", "P4PORT", "P4USER", "P4PASSWD":
			continue
		}
		result = append(result, entry)
	}
	return append(result, "P4CONFIG=", "P4ENVIRO="+os.DevNull)
}

func parse(input string) ([]Record, error) {
	decoder := json.NewDecoder(strings.NewReader(input))
	decoder.UseNumber()
	result := []Record{}
	for {
		var raw map[string]any
		if err := decoder.Decode(&raw); err == io.EOF {
			return result, nil
		} else if err != nil {
			return nil, errors.New("Perforce 返回无效 JSON")
		}
		record := Record{}
		for key, value := range raw {
			record[key] = fmt.Sprint(value)
		}
		for key, value := range record {
			if (key == "Root" || key == "Stream" || strings.HasPrefix(key, "View") || strings.HasPrefix(key, "depotFile") || key == "clientFile") && strings.ContainsRune(value, '\ufffd') {
				return nil, errors.New("Perforce 路径不是有效 UTF-8；请使用 Unicode 服务或 ASCII 路径，拒绝替换文件名")
			}
		}
		severity, _ := strconv.Atoi(record["severity"])
		if severity >= 3 {
			message, _ := diagnostic.Text(record["data"], 4096)
			return nil, errors.New("Perforce: " + message)
		}
		result = append(result, record)
	}
}
