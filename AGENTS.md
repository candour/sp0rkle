# sp0rkle Developer Guide: How to Teach an Old Bot New Tricks

Welcome to the inner sanctum of `sp0rkle`. This bot has survived the transition from perlfu, survived multiple Go versions, and is currently surviving a "long slog" migration away from MongoDB. It's a delicate ecosystem of regex, globals, and IRC protocol quirks.

This guide is intended for humans (and AI agents who have been tasked with its maintenance) to understand how to add new features without causing a catastrophic data divergence or a netsplit.

## 1. Architecture Overview: The Grand Unified Theory of sp0rkle

sp0rkle follows a modular "driver-based" architecture. The core handles the IRC connection, while features are implemented as independent drivers.

### Directory Structure

- **`bot/`**: The Brains. Manages the bot's core state, command sets, line rewriters, and background pollers. If you change this, you might break everything.
- **`collections/`**: The Data Layer. High-level, typesafe abstractions for specific data types (factoids, karma, quotes) built on top of the `db` package.
- **`db/`**: The "Long Slog" Layer. This package handles the dual-writing logic for the MongoDB-to-BoltDB migration. It's designed to keep both databases in sync until we can finally delete the Mongo code.
- **`drivers/`**: The Heart of Features. Each directory here (e.g., `drivers/factdriver/`, `drivers/karmadriver/`) is a self-contained feature driver.
- **`util/`**: The Wizardry. General-purpose helper functions for lexing, string manipulation, time formatting, and some ancillary binaries.

### Events and Drivers

sp0rkle is an event-based bot. The core uses `goirc/client` to handle IRC protocol events, which are then translated into bot-specific events in `bot/handlers.go`.

Drivers are the primary abstraction for adding functionality. They register event handlers and commands during their initialization.

- **`bot.BotHandler`**: A function type `func(*Context)` that processes bot events.
- **Commands**: Registered via `bot.Command(handler, prefix, help)`.
- **Raw Handlers**: Registered via `bot.Handle(handler, events...)`.

### Rewriters (the new "Plugins")

The legacy "plugin" system from the Wiki has been largely subsumed by **Rewriters**. These allow drivers to modify outgoing text before it is sent to IRC.

- **`bot.Rewrite(handler)`**: Registers a function `func(string, *Context) string` to transform outgoing messages (e.g., replacing `$nick` with the user's name).

## 2. Developing New Features (Drivers)

The most common way to extend sp0rkle is by creating a new driver.

### Step 1: Create the Driver
Create a new directory in `drivers/` (e.g., `drivers/weatherdriver/`).

### Step 2: The `Init()` Function
Every driver must have an `Init()` function. This is where you register your commands and handlers with the bot.

```go
func Init() {
    // Register a command: !myfeature <args>
    bot.Command(myHandler, "myfeature", "myfeature <args> -- does something cool")

    // Register a raw handler for IRC events
    bot.Handle(rawHandler, client.PRIVMSG)

    // Register a rewriter (modifies outgoing text)
    bot.Rewrite(myRewriter)
}
```

### Step 3: Register in `main.go`
Your driver won't do anything unless you import it in `main.go` and call its `Init()` function. Yes, this is manual. No, we don't have fancy auto-discovery.

### Step 4: The Handler Logic
Handlers receive a `*bot.Context`. Use it to interact with the world:
- `ctx.Text()`: Returns the message content (with the bot's name and the command prefix already stripped).
- `ctx.ReplyN("format %s", arg)`: Replies to the user with "Nick: format arg".
- `ctx.Storable()`: Returns the sender's `Nick` and `Chan`.

## 3. Dealing with Data: The Database Abstractions

We are migrating from MongoDB to BoltDB. Because we enjoy pain, we write to both.

### `db.Collection`
Never talk to the database directly. Use the abstractions in `db/`.
- `db.Both`: Implements `Collection` and handles the dual-write/read-compare logic.
- `db.K`: Key builder. Keys are composed of elements like `db.S` (string), `db.I` (int), and `db.ID` (bson.ObjectId).

```go
// Example: Creating a key for a specific user's karma
key := db.K{db.S{"nick", "fluffle"}, db.S{"subject", "sp0rkle"}}
```

### `collections/`
Look at existing collections like `collections/factoids/` to see how to wrap the `db` layer into a typesafe API for your driver.

## 4. Testing: Prove Your Feature Works

Tests live alongside the code in `_test.go` files. Since we don't like complex integration tests, we focus on testing the handler logic or utility functions.

### How to Test a Handler (The Lazy Way)

You don't need a real IRC server. Just mock the `bot.Context`. Ensure you add unit tests for **positive paths**, **negative paths** (error handling), and **edge cases**.

```go
func TestMyFeature(t *testing.T) {
    // 1. Create a fake IRC line
    line := &client.Line{
        Nick: "tester",
        Args: []string{"#channel", "myfeature arg1 arg2"},
    }

    // 2. Wrap it in a context
    ctx := &bot.Context{Line: line}

    // 3. Call your handler
    myHandler(ctx)

    // 4. Check results (e.g., did it write to the DB? Did it reply?)
    // Note: To check replies, you might need to mock the IRC connection
    // inside the context, but usually, testing the underlying
    // data-mangling functions is enough.
}
```

See `drivers/factdriver/plugin_test.go` for real-world examples of exercising logic without descending into integration-test hell.

## 5. Pro-Tips and Pitfalls

- **The Global `bot`**: The `bot` package uses a global singleton. It's not "modern Go," but it works.
    - **How to be careful**: Access to the global bot state is protected by a mutex, but your drivers might not be. Use your own locks for shared driver state.
    - **Test Isolation**: Because of the singleton, tests can leak state. Always reset or mock the necessary bot state in `TestMain` or at the start of individual tests.
    - **No Dependency Injection**: You can't easily swap the bot out. Mock the `bot.Context` instead to test your handlers.
- **Data Loss (The Mongo Curse)**: We hate MongoDB. That's why we're migrating. If you find a data mismatch between Mongo and Bolt, the `db.Both` layer will log a warning. Listen to it.
- **Regex is Your Friend (and Enemy)**: Much of sp0rkle's parsing relies on regex. Use `util/lexer.go` if you want to parse complex strings without descending into madness.
- **Pollers**: If your feature needs to do something periodically (like checking an RSS feed), you *could* use `bot.Poll(myPoller)`.
    - **Warning**: Pollers are architecturally flawed. They are tightly coupled to the connection lifecycle and can't be started/stopped arbitrarily. We don't recommend using them in their current state.

## 6. Development Environment

1. Install Go and MongoDB (if you must).
2. Run `go build` in the root.
3. Run the bot: `./sp0rkle --servers irc.yournet.org --nick mybot --channels "#test"`.
4. If you need a database backup, see `backup.sh`. Note: sp0rkle has not lost data in >20 years, but don't tempt fate.

Happy Hacking!
