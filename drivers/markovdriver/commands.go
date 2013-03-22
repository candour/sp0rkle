package markovdriver

import (
	"github.com/fluffle/sp0rkle/bot"
	"github.com/fluffle/sp0rkle/collections/conf"
	chain "github.com/fluffle/sp0rkle/util/markov"
	"strings"
)

func enableMarkov(ctx *bot.Context) {
	conf.Ns(markovNs).String(ctx.Nick, "markov")
	ctx.ReplyN("I'll markov you like I markov'd your mum last night.")
}

func disableMarkov(ctx *bot.Context) {
	conf.Ns(markovNs).Delete(ctx.Nick)
	if err := mc.ClearTag("user:"+ctx.Nick); err != nil {
		ctx.ReplyN("Failed to clear tag: %s", err)
		return
	}
	ctx.ReplyN("Sure, bro, I'll stop.")
}

func randomCmd(ctx *bot.Context) {
	if len(ctx.Text()) == 0 {
		ctx.ReplyN("Be who? Your mum?")
		return
	}

	source := mc.Source("user:" + strings.Fields(ctx.Text())[0])
	if out, err := chain.Generate(source); err == nil {
		ctx.ReplyN("%s would say: %s", ctx.Text(), out)
	} else {
		ctx.ReplyN("markov error: %v", err)
	}
}

func insult(ctx *bot.Context) {
	source := mc.Source("tag:insult")
	if out, err := chain.Generate(source); err == nil {
		if len(ctx.Text()) > 0 {
			ctx.Reply("%s: %s", ctx.Text(), out)
		} else {
			ctx.Reply("%s", out)
		}
	} else {
		ctx.ReplyN("markov error: %v", err)
	}
}

func learn(ctx *bot.Context) {
	s := strings.SplitN(ctx.Text(), " ", 2)
	if len(s) != 2 {
		ctx.ReplyN("I can't learn from you, you're an idiot.")
		return
	}

	// Prepending "tag:" prevents people from learning as "user:foo".
	mc.AddSentence(s[1], "tag:"+s[0])
	if ctx.Public() {
		// Allow large-scale learning via privmsg by not replying there.
		ctx.ReplyN("Ta. You're a fount of knowledge, you are.")
	}
}
