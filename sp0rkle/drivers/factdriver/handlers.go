package factdriver

import (
	"fmt"
	"github.com/fluffle/goevent/event"
	"github.com/fluffle/sp0rkle/lib/factoids"
	"github.com/fluffle/sp0rkle/lib/util"
	"github.com/fluffle/sp0rkle/sp0rkle/base"
	"github.com/fluffle/sp0rkle/sp0rkle/bot"
	"labix.org/v2/mgo/bson"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

func (fd *factoidDriver) RegisterHandlers(r event.EventRegistry) {
	r.AddHandler(bot.NewHandler(fd_privmsg), "bot_privmsg")
	r.AddHandler(bot.NewHandler(fd_action), "bot_action")
}

func fd_privmsg(bot *bot.Sp0rkle, line *base.Line) {
	fd := bot.GetDriver(driverName).(*factoidDriver)

	// If we're not being addressed directly, short-circuit to lookup.
	if !line.Addressed {
		fd_lookup(bot, fd, line)
		return
	}

	nl := line.Copy()
	// Test for various possible courses of action.
	switch {
	// Factoid add: 'key := value' or 'key :is value'
	case util.ContainsAny(nl.Args[1], []string{":=", ":is"}):
		fd_add(bot, fd, nl)

	// Factoid delete: 'forget|delete that' => deletes fd.lastseen[chan]
	case util.HasAnyPrefix(nl.Args[1], []string{"forget that", "delete that"}):
		fd_delete(bot, fd, nl)

	// Factoid replace: 'replace that with' => updates fd.lastseen[chan]
	case util.StripAnyPrefix(&nl.Args[1], []string{"replace that with "}):
		fd_replace(bot, fd, nl)

	// Factoid chance: 'chance of that is' => sets chance of fd.lastseen[chan]
	case util.StripAnyPrefix(&nl.Args[1], []string{"chance of that is "}):
		fd_chance(bot, fd, nl)

	// Factoid literal: 'literal key' => info about factoid
	case util.StripAnyPrefix(&nl.Args[1], []string{"literal "}):
		fd_literal(bot, fd, nl)

	// Factoid search: 'fact search regexp' => list of possible key matches
	case util.StripAnyPrefix(&nl.Args[1], []string{"fact search "}):
		fd_search(bot, fd, nl)

	// Factoid info: 'fact info key' => some information about key
	case util.StripAnyPrefix(&nl.Args[1], []string{"fact info "}):
		fd_info(bot, fd, nl)

	// If we get to here, none of the other FD command possibilities
	// have matched, so try a lookup...
	default:
		fd_lookup(bot, fd, nl)
	}
}

func fd_action(bot *bot.Sp0rkle, line *base.Line) {
	fd := bot.GetDriver(driverName).(*factoidDriver)
	// Actions just trigger a lookup.
	fd_lookup(bot, fd, line)
}

func fd_add(bot *bot.Sp0rkle, fd *factoidDriver, line *base.Line) {
	var key, val string
	if strings.Index(line.Args[1], ":=") != -1 {
		kv := strings.SplitN(line.Args[1], ":=", 2)
		key = ToKey(kv[0], false)
		val = strings.TrimSpace(kv[1])
	} else {
		// we use :is to add val = "key is val"
		kv := strings.SplitN(line.Args[1], ":is", 2)
		key = ToKey(kv[0], false)
		val = strings.Join([]string{strings.TrimSpace(kv[0]),
			"is", strings.TrimSpace(kv[1])}, " ")
	}
	n, c := line.Storable()
	fact := factoids.NewFactoid(key, val, n, c)
	if err := fd.Insert(fact); err == nil {
		count := fd.GetCount(key)
		bot.ReplyN(line, "Woo, I now know %d things about '%s'.", count, key)
	} else {
		bot.ReplyN(line, "Error storing factoid: %s.", err)
	}
}

func fd_chance(bot *bot.Sp0rkle, fd *factoidDriver, line *base.Line) {
	str := strings.TrimSpace(line.Args[1])
	var chance float64

	if strings.HasSuffix(str, "%") {
		// Handle 'chance of that is \d+%'
		if i, err := strconv.Atoi(str[:len(str)-1]); err != nil {
			bot.ReplyN(line, "'%s' didn't look like a % chance to me.", str)
			return
		} else {
			chance = float64(i) / 100
		}
	} else {
		// Assume the chance is a floating point number.
		if c, err := strconv.ParseFloat(str, 64); err != nil {
			bot.ReplyN(line, "'%s' didn't look like a chance to me.", str)
			return
		} else {
			chance = c
		}
	}

	// Make sure the chance we've parsed lies in (0.0,1.0]
	if chance > 1.0 || chance <= 0.0 {
		bot.ReplyN(line, "'%s' was outside possible chance ranges.", str)
		return
	}

	// Retrieve last seen ObjectId, replace with ""
	ls := fd.Lastseen(line.Args[0], "")
	// ok, we're good to update the chance.
	if fact := fd.GetById(ls); fact != nil {
		// Store the old chance, update with the new
		old := fact.Chance
		fact.Chance = chance
		// Update the Modified field
		fact.Modify(line.Storable())
		// And store the new factoid data
		if err := fd.Update(bson.M{"_id": ls}, fact); err == nil {
			bot.ReplyN(line, "'%s' was at %.0f%% chance, now is at %.0f%%.",
				fact.Key, old*100, chance*100)
		} else {
			bot.ReplyN(line, "I failed to replace '%s': %s", fact.Key, err)
		}
	} else {
		bot.ReplyN(line, "Whatever that was, I've already forgotten it.")
	}
}

func fd_delete(bot *bot.Sp0rkle, fd *factoidDriver, line *base.Line) {
	// Get fresh state on the last seen factoid.
	ls := fd.Lastseen(line.Args[0], "")
	if fact := fd.GetById(ls); fact != nil {
		if err := fd.Remove(bson.M{"_id": ls}); err == nil {
			bot.ReplyN(line, "I forgot that '%s' was '%s'.",
				fact.Key, fact.Value)
		} else {
			bot.ReplyN(line, "I failed to forget '%s': %s", fact.Key, err)
		}
	} else {
		bot.ReplyN(line, "Whatever that was, I've already forgotten it.")
	}
}

func fd_info(bot *bot.Sp0rkle, fd *factoidDriver, line *base.Line) {
	key := ToKey(line.Args[1], false)
	count := fd.GetCount(key)
	if count == 0 {
		bot.ReplyN(line, "I don't know anything about '%s'.", key)
		return
	}
	msgs := make([]string, 0, 10)
	if key == "" {
		msgs = append(msgs, fmt.Sprintf("In total, I know %d things.", count))
	} else {
		msgs = append(msgs, fmt.Sprintf("I know %d things about '%s'.",
			count, key))
	}
	if created := fd.GetLast("created", key); created != nil {
		c := created.Created
		msgs = append(msgs, "A factoid")
		if key != "" {
			msgs = append(msgs, fmt.Sprintf("for '%s'", key))
		}
		msgs = append(msgs, fmt.Sprintf("was last created on %s by %s,",
			c.Timestamp.Format(time.ANSIC), c.Nick))
	}
	if modified := fd.GetLast("modified", key); modified != nil {
		m := modified.Modified
		msgs = append(msgs, fmt.Sprintf("modified on %s by %s,",
			m.Timestamp.Format(time.ANSIC), m.Nick))
	}
	if accessed := fd.GetLast("accessed", key); accessed != nil {
		a := accessed.Accessed
		msgs = append(msgs, fmt.Sprintf("and accessed on %s by %s.",
			a.Timestamp.Format(time.ANSIC), a.Nick))
	}
	if info := fd.InfoMR(key); info != nil {
		if key == "" {
			msgs = append(msgs, "These factoids have")
		} else {
			msgs = append(msgs, fmt.Sprintf("'%s' has", key))
		}
		msgs = append(msgs, fmt.Sprintf(
			"been modified %d times and accessed %d times.",
			info.Modified, info.Accessed))
	}
	bot.ReplyN(line, "%s", strings.Join(msgs, " "))
}

func fd_literal(bot *bot.Sp0rkle, fd *factoidDriver, line *base.Line) {
	key := ToKey(line.Args[1], false)
	if count := fd.GetCount(key); count == 0 {
		bot.ReplyN(line, "I don't know anything about '%s'.", key)
		return
	} else if count > 10 && strings.HasPrefix(line.Args[0], "#") {
		bot.ReplyN(line, "I know too much about '%s', ask me privately.", key)
		return
	}

	// Temporarily turn off flood protection cos we could be spamming a bit.
	bot.Conn.Flood = true
	defer func() { bot.Conn.Flood = false }()
	// Passing an anonymous function to For makes it a little hard to abstract
	// away in lib/factoids. Fortunately this is something of a one-off.
	var fact *factoids.Factoid
	f := func() error {
		if fact != nil {
			bot.ReplyN(line, "[%3.0f%%] %s", fact.Chance*100, fact.Value)
		}
		return nil
	}
	if err := fd.Find(bson.M{"key": key}).For(&fact, f); err != nil {
		bot.ReplyN(line, "Something literally went wrong: %s", err)
	}
}

func fd_lookup(bot *bot.Sp0rkle, fd *factoidDriver, line *base.Line) {
	// Only perform extra prefix removal if we weren't addressed directly
	key := ToKey(line.Args[1], !line.Addressed)
	var fact *factoids.Factoid

	if fact = fd.GetPseudoRand(key); fact == nil && line.Cmd == "ACTION" {
		// Support sp0rkle's habit of stripping off it's own nick
		// but only for actions, not privmsgs.
		if strings.HasSuffix(key, bot.Conn.Me.Nick) {
			key = strings.TrimSpace(key[:len(key)-len(bot.Conn.Me.Nick)])
			fact = fd.GetPseudoRand(key)
		}
	}
	if fact == nil {
		return
	}
	// Chance is used to limit the rate of factoid replies for things
	// people say a lot, like smilies, or 'lol', or 'i love the peen'.
	chance := fact.Chance
	if key == "" {
		// This is doing a "random" lookup, triggered by someone typing in
		// something entirely composed of the chars stripped by ToKey().
		// To avoid making this too spammy, forcibly limit the chance to 40%.
		chance = 0.4
	}
	if rand.Float64() < chance {
		// Store this as the last seen factoid
		fd.Lastseen(line.Args[0], fact.Id)
		// Update the Accessed field
		// TODO(fluffle): fd should take care of updating Accessed internally
		fact.Access(line.Storable())
		// And store the new factoid data
		if err := fd.Update(bson.M{"_id": fact.Id}, fact); err != nil {
			bot.ReplyN(line, "I failed to update '%s' (%s): %s ",
				fact.Key, fact.Id, err)
		}

		switch fact.Type {
		case factoids.F_ACTION:
			bot.Do(line, fact.Value)
		default:
			bot.Reply(line, fact.Value)
		}
	}
}

func fd_replace(bot *bot.Sp0rkle, fd *factoidDriver, line *base.Line) {
	ls := fd.Lastseen(line.Args[0], "")
	if fact := fd.GetById(ls); fact != nil {
		// Store the old factoid value
		old := fact.Value
		// Replace the value with the new one
		fact.Value = strings.TrimSpace(line.Args[1])
		// Update the Modified field
		fact.Modify(line.Storable())
		// And store the new factoid data
		if err := fd.Update(bson.M{"_id": ls}, fact); err == nil {
			bot.ReplyN(line, "'%s' was '%s', now is '%s'.",
				fact.Key, old, fact.Value)
		} else {
			bot.ReplyN(line, "I failed to replace '%s': %s", fact.Key, err)
		}
	} else {
		bot.ReplyN(line, "Whatever that was, I've already forgotten it.")
	}
}

func fd_search(bot *bot.Sp0rkle, fd *factoidDriver, line *base.Line) {
	keys := fd.GetKeysMatching(line.Args[1])
	if keys == nil || len(keys) == 0 {
		bot.ReplyN(line, "I couldn't think of anything matching '%s'.",
			line.Args[0])
		return
	}
	// RESULTS.
	count := len(keys)
	if count > 10 {
		res := strings.Join(keys[:10], "', '")
		bot.ReplyN(line,
			"I found %d keys matching '%s', here's the first 10: '%s'.",
			count, line.Args[1], res)
	} else {
		res := strings.Join(keys, "', '")
		bot.ReplyN(line,
			"%s: I found %d keys matching '%s', here they are: '%s'.",
			count, line.Args[1], res)
	}
}
