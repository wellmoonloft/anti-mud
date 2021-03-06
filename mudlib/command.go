package mudlib

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

type command struct {
	name             string
	minArgs, maxArgs int
	usage            []string
	do               func(client, []string) (*string, *message)
}

var commands = make(map[string]command)

func init() {
	commands["退出"] = command{
		minArgs: 0,
		maxArgs: 0,
		usage:   []string{""},
		do: func(cl client, args []string) (*string, *message) {
			io.WriteString(cl.conn, "再见!\n")
			cl.conn.Close()
			return nil, &message{
				from:        cl,
				message:     "",
				messageType: messageTypeQuit,
			}
		},
	}
	commands["说"] = command{
		minArgs: 1,
		maxArgs: -1,
		usage:   []string{"<message>"},
		do: func(cl client, args []string) (*string, *message) {
			return nil, &message{
				message:     strings.Join(args, " "),
				messageType: messageTypeSay,
			}
		},
	}

	commands["密语"] = command{
		minArgs: 2,
		maxArgs: -1,
		usage:   []string{"<player> <message>"},
		do: func(cl client, args []string) (*string, *message) {
			player, err := players.get(args[0])
			if err != nil {
				ret := fmt.Sprintf("无法找到玩家。 %q\n", args[0])
				return &ret, nil
			}
			if conn, _ := player.isConnected(); conn {
				return nil, &message{
					to:          player.nickname,
					message:     strings.Join(args[1:], " "),
					messageType: messageTypeTell,
				}
			}
			ret := fmt.Sprintf("%q 已经不在线。\n", args[0])
			return &ret, nil
		},
	}
	commands["我"] = command{
		minArgs: 1,
		maxArgs: -1,
		usage:   []string{"<emotes>"},
		do: func(cl client, args []string) (*string, *message) {
			return nil, &message{
				message:     strings.Join(args, " "),
				messageType: messageTypeEmote,
			}
		},
	}
	commands["大喊"] = command{
		minArgs: 1,
		maxArgs: -1,
		usage:   []string{"<message>"},
		do: func(cl client, args []string) (*string, *message) {
			return nil, &message{
				message:     strings.Join(args, " "),
				messageType: messageTypeShout,
			}
		},
	}
	commands["在线玩家"] = command{
		minArgs: 0,
		maxArgs: 0,
		usage:   []string{""},
		do: func(cl client, args []string) (*string, *message) {
			connected := getConnected()
			ret := fmt.Sprintf("现在有 %d 个玩家在线:\n  %s\n", len(connected), strings.Join(getConnected(), ", "))
			return &ret, nil
		},
	}
	commands["查玩家"] = command{
		minArgs: 1,
		maxArgs: 1,
		usage:   []string{"<player>"},
		do: func(cl client, args []string) (*string, *message) {
			toPrint := ""
			if player, err := players.get(args[0]); err == nil {
				toPrint = setFg(colorWhite, fmt.Sprintf("%+v ", player.finger()))
				if c, _ := player.isConnected(); c {
					toPrint += setFgBold(colorGreen, "[online]\n")
				} else {
					toPrint += setFgBold(colorRed, "[offline]\n")
				}
			} else {
				toPrint = fmt.Sprintf("找不到玩家 %q.\n", args[0])
			}
			return &toPrint, nil
		},
	}
	commands["看"] = command{
		minArgs: 0,
		maxArgs: -1,
		usage:   []string{"", "<object>", "<player>"},
		do: func(cl client, args []string) (*string, *message) {
			switch len(args) {
			case 0:
				// room look
				player, err := players.get(cl.player)
				if err != nil {
					errorLog.Fatalf("%+v", err)
				}
				if room, err := rooms.get(player.room); err == nil {
					desc := room.describe(*player)
					return &desc, nil
				}
				// TODO: handle limbo
				errorLog.Printf("%+v in limbo.\n", player)
				desc := "You're in limbo.\n"
				return &desc, nil
			default:
				// TODO: look at objects/players
				return nil, nil
			}
			return nil, nil
		},
	}
	commands["帮助"] = command{
		minArgs: 0,
		maxArgs: 1,
		usage:   []string{"", "<command>"},
		do: func(cl client, args []string) (*string, *message) {
			switch len(args) {
			case 0:
				ret := fmt.Sprintf("有效命令:\n")
				keys := []string{}
				for k := range commands {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				ret += fmt.Sprintf("  %s\n", strings.Join(keys, ", "))
				return &ret, nil
			case 1:
				if c, ok := commands[args[0]]; ok {
					// Use abstracted usage print method
					ret := c.printUsage(args[0])
					return &ret, nil
				}
				ret := fmt.Sprintf("Unknown command %q.\n", args[0])
				return &ret, nil
			}
			return nil, nil
		},
	}
	commands["往"] = command{
		minArgs: 1,
		maxArgs: 1,
		usage:   []string{"<direction>"},
		do: func(cl client, args []string) (*string, *message) {
			player, err := players.get(cl.player)
			if err != nil {
				errorLog.Fatalf("%+v", err)
			}
			room, err := rooms.get(player.room)
			if err != nil {
				errorLog.Printf("Player %+v is in limbo.\n", player)
				ret := fmt.Sprintf("你不能往 %q: %+v\n", args[0], err)
				return &ret, nil
			}
			if err := player.toRoom(cl, room.exits[args[0]]); err != nil {
				ret := fmt.Sprintf("你不能往 %q: %+v\n", args[0], err)
				return &ret, nil
			}
			ret := setFg(colorCyan, fmt.Sprintf("你往 %s.\n", args[0]))
			// Force a look on move.
			lookRet, msg := commands["看"].do(cl, []string{})
			ret += *lookRet
			return &ret, msg
		},
	}
}

func (c command) printUsage(cmd string) string {
	usage := "Usage:\n"
	for _, s := range c.usage {
		usage += fmt.Sprintf("  /%s %s\n", cmd, s)
	}
	return usage
}

func doCommand(cl client, cmd string, args []string) error {
	if c, ok := commands[cmd[1:]]; ok {
		if (c.minArgs != -1 && len(args) < c.minArgs) || (c.maxArgs != -1 && len(args) > c.maxArgs) {
			io.WriteString(cl.conn, c.printUsage(cmd))
			return nil
		}
		toPrint, msg := c.do(cl, args)
		if toPrint != nil {
			io.WriteString(cl.conn, *toPrint)
		}
		if msg != nil {
			msg.from = cl
			msgchan <- *msg
		}
		return nil
	}
	io.WriteString(cl.conn, " (尝试用命令 \"/帮助\")\n")
	return fmt.Errorf("没有这个命令 %q", cmd)
}
