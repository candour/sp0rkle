package reminddriver

import (
	"github.com/fluffle/sp0rkle/bot"
	"github.com/fluffle/sp0rkle/collections/reminders"
	"github.com/fluffle/sp0rkle/util/datetime"
	"labix.org/v2/mgo/bson"
	"strconv"
	"strings"
	"time"
)

// remind del
func del(ctx *bot.Context) {
	list, ok := listed[ctx.Nick]
	if !ok {
		ctx.ReplyN("Please use 'remind list' first, " +
			"to be sure of what you're deleting.")

		return
	}
	idx, err := strconv.Atoi(ctx.Text())
	if err != nil || idx > len(list) || idx <= 0 {
		ctx.ReplyN("Invalid reminder index '%s'", ctx.Text())
		return
	}
	idx--
	Forget(list[idx], true)
	delete(listed, ctx.Nick)
	ctx.ReplyN("I'll forget that one, then...")
}

// remind list
func list(ctx *bot.Context) {
	r := rc.RemindersFor(ctx.Nick)
	c := len(r)
	if c == 0 {
		ctx.ReplyN("You have no reminders set.")
		return
	}
	if c > 5 && ctx.Public() {
		ctx.ReplyN("You've got lots of reminders, ask me privately.")
		return
	}
	// Save an ordered list of ObjectIds for easy reminder deletion
	ctx.ReplyN("You have %d reminders set:", c)
	list := make([]bson.ObjectId, c)
	for i := range r {
		ctx.Reply("%d: %s", i+1, r[i].List(ctx.Nick))
		list[i] = r[i].Id
	}
	listed[ctx.Nick] = list
}

// remind 
func set(ctx *bot.Context) {
	// s == <target> <reminder> in|at|on <time>
	s := strings.Fields(ctx.Text())
	if len(s) < 4 {
		ctx.ReplyN("Invalid remind syntax. Sucka.")
		return
	}
	at, ok, reminder, timestr := time.Now(), false, "", ""
	for i := 1; i+1 < len(s); i++ {
		lc := strings.ToLower(s[i])
		if lc == "in" || lc == "at" || lc == "on" {
			reminder = strings.Join(s[1:i], " ")
			timestr = strings.ToLower(strings.Join(s[i+1:], " "))
			// TODO(fluffle): surface better errors from datetime.Parse
			at, ok = datetime.Parse(timestr)
			if ok {
				break
			}
		}
	}
	if timestr == "" {
		ctx.ReplyN("Invalid remind syntax. Sucka.")
		return
	}
	if !ok {
		ctx.ReplyN("Couldn't parse time string '%s'", timestr)
		return
	}
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	if at.Before(now) && at.After(start) {
		// Perform some basic hacky corrections before giving up
		if strings.Contains(timestr, "am") || strings.Contains(timestr, "pm") {
			at = at.Add(24 * time.Hour)
		} else {
			at = at.Add(12 * time.Hour)
		}
	}
	if at.Before(now) {
		ctx.ReplyN("Time '%s' is in the past.", timestr)
		return
	}
	n, c := ctx.Storable()
	// TODO(fluffle): Use state tracking! And do this better.
	t := bot.Nick(s[0])
	if t.Lower() == strings.ToLower(ctx.Nick) ||
		t.Lower() == "me" {
		t = n
	}
	r := reminders.NewReminder(reminder, at, t, n, c)
	if err := rc.Insert(r); err != nil {
		ctx.ReplyN("Error saving reminder: %v", err)
		return
	}
	// Any previously-generated list of reminders is now obsolete.
	delete(listed, ctx.Nick)
	ctx.ReplyN("%s", r.Acknowledge())
	Remind(r, ctx)
}

// tell
func tell(ctx *bot.Context) {
	// s == <target> <stuff>
	txt := ctx.Text()
	idx := strings.Index(txt, " ")
	if idx == -1 {
		ctx.ReplyN("Tell who what?")
		return
	}
	tell := txt[idx+1:]
	n, c := ctx.Storable()
	t := bot.Nick(txt[:idx])
	if t.Lower() == strings.ToLower(ctx.Nick) ||
		t.Lower() == "me" {
		ctx.ReplyN("You're a dick. Oh, wait, that wasn't *quite* it...")
		return
	}
	r := reminders.NewTell(tell, t, n, c)
	if err := rc.Insert(r); err != nil {
		ctx.ReplyN("Error saving tell: %v", err)
		return
	}
	// Any previously-generated list of reminders is now obsolete.
	delete(listed, ctx.Nick)
	ctx.ReplyN("%s", r.Acknowledge())
}
