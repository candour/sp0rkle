package reminddriver

import (
	"github.com/fluffle/goirc/client"
	"github.com/fluffle/golog/logging"
	"github.com/fluffle/sp0rkle/bot"
	"github.com/fluffle/sp0rkle/collections/reminders"
	"labix.org/v2/mgo/bson"
	"time"
)

// We use the reminders collection
var rc *reminders.Collection

// We need to be able to kill reminder goroutines
var running = map[bson.ObjectId]chan struct{}{}

// And it's useful to index them for deletion per-person
var listed = map[string][]bson.ObjectId{}

func Init() {
	rc = reminders.Init()

	// Set up the handlers and commands.
	bot.Handle(load, client.CONNECTED)
	bot.Handle(unload, client.DISCONNECTED)
	bot.Handle(tellCheck,
		client.PRIVMSG, client.ACTION, client.JOIN, client.NICK)

	bot.Command(tell, "tell", "tell <nick> <msg>  -- "+
		"Stores a message for the (absent) nick.")
	bot.Command(list, "remind list",
		"remind list  -- Lists reminders set by or for your nick.")
	bot.Command(del, "remind del",
		"remind del <N>  -- Deletes (previously listed) reminder N.")
	bot.Command(set, "remind", "remind <nick> <msg> "+
		"in|at|on <time>  -- Reminds nick about msg at time.")
}

func Remind(r *reminders.Reminder, ctx *bot.Context) {
	delta := r.RemindAt.Sub(time.Now())
	if delta < 0 {
		return
	}
	c := make(chan struct{})
	running[r.Id] = c
	go func() {
		select {
		case <-time.After(delta):
			ctx.Privmsg(string(r.Chan), r.Reply())
			// TODO(fluffle): Tie this into state tracking properly.
			ctx.Privmsg(string(r.Target), r.Reply())
			Forget(r.Id, false)
		case <-c:
			return
		}
	}()
}

func Forget(id bson.ObjectId, stop bool) {
	c, ok := running[id]
	if !ok { return }
	delete(running, id)
	if stop {
		c <- struct{}{}
	}
	if err := rc.RemoveId(id); err != nil {
		logging.Error("Failure removing reminder %s: %v", id, err)
	}
}
