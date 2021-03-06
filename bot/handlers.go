package bot

import (
	"flag"
	"fmt"
	"github.com/fluffle/golog/logging"
	"os/exec"
	"strings"
)

var (
	channels *string = flag.String("channels", "#sp0rklf",
		"Comma-separated list of channels to join, defaults to '#sp0rklf'")
	rebuilder *string = flag.String("rebuilder", "",
		"Nick[:password] to accept rebuild command from.")
	oper *string = flag.String("oper", "",
		"user:password for server OPER command on connect.")
	vhost *string = flag.String("vhost", "",
		"user:password for server VHOST command on connect.")
)

func connected(ctx *Context) {
	// Set bot mode to keep people informed.
	ctx.conn.Mode(ctx.Me(), "+B")
	if *oper != "" {
		up := strings.SplitN(*oper, ":", 2)
		if len(up) == 2 {
			ctx.conn.Oper(up[0], up[1])
		}
	}
	if *vhost != "" {
		up := strings.SplitN(*vhost, ":", 2)
		if len(up) == 2 {
			ctx.conn.VHost(up[0], up[1])
		}
	}
	for _, c := range strings.Split(*channels, ",") {
		logging.Info("Joining %s on startup.\n", c)
		ctx.conn.Join(c)
	}
}

func rebuild(ctx *Context) {
	if !check_rebuilder("rebuild", ctx) {
		return
	}

	// Ok, we should be good to rebuild now.
	logging.Info("Beginning rebuild")
	ctx.conn.Notice(ctx.Nick, "Beginning rebuild")
	cmd := exec.Command("go", "get", "-u", "github.com/fluffle/sp0rkle")
	out, err := cmd.CombinedOutput()
	logging.Info("Output from go get:\n%s", out)
	if err != nil {
		ctx.conn.Notice(ctx.Nick, fmt.Sprintf("Rebuild failed: %s", err))
		for _, l := range strings.Split(string(out), "\n") {
			ctx.conn.Notice(ctx.Nick, l)
		}
		return
	}
	bot.servers.Shutdown(true)
}

func shutdown(ctx *Context) {
	if check_rebuilder("shutdown", ctx) {
		bot.servers.Shutdown(false)
	}
}

func check_rebuilder(cmd string, ctx *Context) bool {
	s := strings.Split(*rebuilder, ":")
	if s[0] == "" || s[0] != ctx.Nick || !strings.HasPrefix(ctx.Text(), cmd) {
		return false
	}
	if len(s) > 1 && ctx.Text() != fmt.Sprintf("%s %s", cmd, s[1]) {
		return false
	}
	return true
}
