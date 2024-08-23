package main

/*
Запуск скрипта на кнопку F12.
1-е действие. После прожатия кнопки курсор мышки передвигается на координаты x\y и делает клик левой копкой мыши.
2-е действие. Курсор двигается во вторую координату x\y и делает клик левой кнопкой мыши.
3-е действие. Курсор двигается в третью координату x\y и делает двойной клик левой кнопкой мыши.
После чего скрипт останавливает свое действие и ждет повторного нажатия кнопки F12
*/

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/go-vgo/robotgo"
	hook "github.com/robotn/gohook"
	"github.com/urfave/cli/v2"
)

func init() {
	hook.Keycode["f11"] = 87
	hook.Keycode["f12"] = 88
}

/*
{"id":4,"When":"2024-08-23T15:30:22.7354134+03:00","mask":0,"reserved":0,"keycode":68,"rawcode":121,"keychar":65535,"button":0,"clicks":0,"x":0,"y":0,"amount":0,"rotation":0,"direction":0}
{"id":5,"When":"2024-08-23T15:30:22.7855361+03:00","mask":0,"reserved":0,"keycode":68,"rawcode":121,"keychar":65535,"button":0,"clicks":0,"x":0,"y":0,"amount":0,"rotation":0,"direction":0}
{"id":4,"When":"2024-08-23T15:30:23.0368941+03:00","mask":0,"reserved":0,"keycode":87,"rawcode":122,"keychar":65535,"button":0,"clicks":0,"x":0,"y":0,"amount":0,"rotation":0,"direction":0}
{"id":5,"When":"2024-08-23T15:30:23.0873316+03:00","mask":0,"reserved":0,"keycode":87,"rawcode":122,"keychar":65535,"button":0,"clicks":0,"x":0,"y":0,"amount":0,"rotation":0,"direction":0}
{"id":4,"When":"2024-08-23T15:30:24.1420402+03:00","mask":0,"reserved":0,"keycode":88,"rawcode":123,"keychar":65535,"button":0,"clicks":0,"x":0,"y":0,"amount":0,"rotation":0,"direction":0}
{"id":5,"When":"2024-08-23T15:30:24.2428318+03:00","mask":0,"reserved":0,"keycode":88,"rawcode":123,"keychar":65535,"button":0,"clicks":0,"x":0,"y":0,"amount":0,"rotation":0,"direction":0}
*/
func main() {
	s := hook.Start()

	fmt.Println("--- Нажмите ctrl + shift + q для выхода из скрипта ---")
	hook.Register(hook.KeyDown, []string{"ctrl", "shift", "q"}, func(e hook.Event) {
		log.Println("выход")
		hook.End()
	})

	app := &cli.App{
		Name: "clicker",
		Args: true,
		Usage: `Для работы скрипта требуется указать команды для выполнения. 
Передать первым аргументом команду следующего вида: "x:y:t;"
 - "800:900:0;" - команда читается как x:y:t;, где x- точка координат X, y - точка координат Y, t - тип клика (0 - одинарный, 1 - двойной), ; - разделитель действий
 - "800:900:0;777:111:1;" - действия в команде можно комбинировать через точку запятой
`,
		UsageText:   "clicker [command options] [arguments...]",
		Description: "Пример команды: clicker -k=f12 -r=1 \"300:400:1;200:100:0;\"",
		Flags: []cli.Flag{
			&cli.StringSliceFlag{
				Name:    "key",
				Usage:   "кнопка для запуска скрипта, можно указать комбинацию из нескольких кнопок, например комбинация ctrl+shift+w будет выглядеть так \"-k=ctrl -k=shift -k=w\"",
				Aliases: []string{"k"},
				Value:   cli.NewStringSlice("f12"),
			},
			&cli.IntFlag{
				Name:    "repeat",
				Aliases: []string{"r"},
				Usage:   "сколько повторений команды требуется при запуске скрипта",
				Value:   1,
			},
			&cli.IntFlag{
				Name:    "wait",
				Aliases: []string{"w"},
				Usage:   "сколько миллисекунд нужно ждать между перемещением мыши и кликом",
				Value:   50,
			},
		},
		Action: func(ctx *cli.Context) (err error) {
			defer func() {
				if err != nil {
					err = fmt.Errorf("ошибка выполнения скрипта: %w", err)

				}
			}()

			cfg := &cfg{
				cmd_str:    ctx.Args().First(),
				keys_start: ctx.StringSlice("key"),
				repeat:     ctx.Int("repeat"),
			}
			if err := cfg.validate(); err != nil {
				return fmt.Errorf("неверная конфигурация: %w", err)
			}
			cmds, err := parse_cmd(cfg.cmd_str)
			if err != nil {
				return err
			}

			fmt.Printf("--- Нажмите %s для запуска скрипта ---\n", strings.Join(cfg.keys_start, " + "))
			once := &atomic.Bool{}
			for _, op := range []uint8{hook.KeyDown, hook.KeyHold, hook.KeyUp} {
				hook.Register(op, cfg.keys_start, func(e hook.Event) {
					if !once.CompareAndSwap(false, true) {
						return
					}
					defer once.Store(false)
					for i := 0; i < cfg.repeat; i++ {
						for _, cmd := range cmds {
							cmd.exec(cfg.wait)
							robotgo.MilliSleep(cfg.wait)
						}
					}
				})
			}

			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Println(err)
	}
	<-hook.Process(s)
}

func parse_cmd(str string) ([]*command, error) {
	parts := strings.Split(strings.TrimSpace(str), ";")
	cmds := make([]*command, len(parts))
	for i, part := range parts {
		params := strings.Split(strings.TrimSpace(part), ":")
		cmd := &command{
			x: sti(get_param(params, 0)),
			y: sti(get_param(params, 1)),
			t: sti(get_param(params, 2)),
		}
		if err := cmd.validate(); err != nil {
			return nil, fmt.Errorf("команда \"%s\" неверная: %w", part, err)
		}
		cmds[i] = cmd
	}
	return cmds, nil
}

func get_param(params []string, n int) string {
	if len(params) > n {
		return strings.TrimSpace(params[n])
	}
	return ""
}

func sti(s string) int {
	if s == "" {
		return 0
	}
	i, err := strconv.Atoi(s)
	if err != nil {
		panic(err)
	}
	return i
}

type command struct {
	x, y, t int
}

func (c *command) validate() error {
	if c.x < 0 {
		return fmt.Errorf("точка x не может быть отрицательной")
	}
	if c.y < 0 {
		return fmt.Errorf("точка y не может быть отрицательной")
	}
	if c.t != 0 && c.t != 1 {
		return fmt.Errorf("клик может иметь значения: 0 - обычный клик ЛКМ, 1 - двойной клик ЛКМ")
	}
	return nil
}

func (c *command) exec(wait int) {
	robotgo.Move(c.x, c.y)
	robotgo.MilliSleep(wait)
	robotgo.Click("left", c.t == 1)
}

type cfg struct {
	cmd_str    string
	keys_start []string
	repeat     int
	wait       int
}

var rex = regexp.MustCompile(`(?m)(\d+:\d+:[0-1]+;?)`)

func (c *cfg) validate() error {
	c.cmd_str = strings.TrimSpace(c.cmd_str)
	for i, k := range c.keys_start {
		c.keys_start[i] = strings.TrimSpace(k)
	}

	if c.cmd_str == "" {
		return fmt.Errorf("команда не задана")
	} else {
		if !rex.MatchString(c.cmd_str) {
			return fmt.Errorf("неверная команда, передайте команду вида \"x:y:t;\", где x - ось x, y - ось y, t - тип клика (0 - одинарный, 1 - двойной), \";\" - разделитель команд, пример команды из 2-ч действий  - \"543:7657:1;1224:4121:0;\"")
		}
	}
	if len(c.keys_start) == 0 {
		return fmt.Errorf("кнопка запуска не задана")
	} else {
		for _, k := range c.keys_start {
			if _, ok := hook.Keycode[k]; !ok {
				return fmt.Errorf("кнопка запуска \"%s\" не поддерживается, поддерживаемые значения: %s", k, allKeysStr())
			}
		}
	}
	if c.repeat <= 0 {
		return fmt.Errorf("повтор команды не может быть меньше 1")
	}
	if c.wait < 0 {
		return fmt.Errorf("ожидание между перемещением мыши и кликом не может быть меньше 0")
	}
	return nil
}

func allKeysStr() string {
	keys := make([]string, 0, len(hook.Keycode))
	for k, _ := range hook.Keycode {
		keys = append(keys, k)
	}

	slices.Sort(keys)
	b := ""
	for _, k := range keys {
		b = fmt.Sprintf("%s,\"%s\"", b, k)
	}

	return strings.Trim(b, ",")
}
