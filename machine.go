package wmfw

import (
	"bytes"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/xyzj/gopsu"
)

func machineCode() string {
	var b bytes.Buffer
	b.WriteString("======= start =======\n")
	cmd := exec.Command("hostname")
	bb, _ := cmd.CombinedOutput()
	b.WriteString(gopsu.OSNAME + " " + gopsu.OSARCH + " " + strings.TrimSpace(gopsu.String(bb)) + " " + strconv.Itoa(runtime.NumCPU()) + "\n")
	ss := make([]string, 0)
	if nis, err := net.Interfaces(); err != nil {
		panic("Error:" + err.Error())
	} else {
		for _, ni := range nis {
			n := strings.ToLower(ni.Name)
			if strings.Contains(n, "lo") || strings.HasPrefix(n, "v") || strings.HasPrefix(n, "t") || strings.HasPrefix(n, "d") || strings.HasPrefix(n, "is") {
				continue
			}
			ss = append(ss, ni.HardwareAddr.String())
		}
	}
	if len(ss) > 0 {
		sort.Slice(ss, func(i int, j int) bool {
			return ss[i] < ss[j]
		})
	}
	for _, v := range ss {
		b.WriteString(v + "\n")
	}
	b.WriteString("======= end =======\n")
	return gopsu.GetMD5(b.String())
}

func (fw *WMFrameWorkV2) checkMachine() {
	home, _ := os.UserConfigDir()
	mfiles := []string{filepath.Join(home, ".firstrun"), filepath.Join(gopsu.JoinPathFromHere(), ".firstrun")}
	for _, mfile := range mfiles {
		// 读取文件错误继续尝试下一个
		if b, err := ioutil.ReadFile(mfile); err == nil {
			a := gopsu.DecodeString(gopsu.String(b))
			if a == "I will never be a memory" || a == machineCode() { // 通过
				return
			}
			println("wrong machine.")
			os.Exit(21)
		}
	}
	// 所有文件失败
	println("wrong machine.")
	os.Exit(21)
}
