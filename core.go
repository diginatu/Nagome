package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/diginatu/nagome/nicolive"
)

type plugin struct {
	Name        string
	Description string
	Version     string
	Auther      string
	Exec        string
	Method      string
	Nagomever   string
	Depends     []string
	Rw          *bufio.ReadWriter
	FlushTm     *time.Timer
}

func (pl *plugin) depend(pln string) bool {
	f := false
	for _, d := range pl.Depends {
		if d == pln {
			f = true
			break
		}
	}
	return f
}

// commentEventEmit will receive comment events and emits commentViewer events.
type commentEventEmit struct {
	cv *commentViewer
}

func (der commentEventEmit) Proceed(ev *nicolive.Event) {
	var content []byte

	switch ev.Type {
	case nicolive.EventTypeGot:
		content, _ = json.Marshal(ev.Content.(nicolive.Comment))
	default:
		Logger.Println(ev.String())
	}
	der.cv.Evch <- &Message{
		Domain:  DomainNagome,
		Func:    FuncComment,
		Command: CommCommentGot,
		Content: content,
	}
}

// A commentViewer is a pair of an Account and a LiveWaku.
type commentViewer struct {
	Ac   *nicolive.Account
	Lw   *nicolive.LiveWaku
	Cmm  *nicolive.CommentConnection
	Pgns []*plugin
	Evch chan *Message
	Quit chan struct{}
}

func (cv *commentViewer) runCommentViewer() {
	var wg sync.WaitGroup

	numProcSendEvent := 5
	wg.Add(numProcSendEvent)
	for i := 0; i < numProcSendEvent; i++ {
		go cv.sendPluginEvent(i, &wg)
	}

	wg.Add(len(cv.Pgns))
	for i, pg := range cv.Pgns {
		Logger.Println(pg.Name)
		go cv.readPluginMes(i, &wg)
	}

	wg.Wait()

	return
}

func (cv *commentViewer) readPluginMes(n int, wg *sync.WaitGroup) {
	defer wg.Done()
	decoded := make(chan bool)

	dec := json.NewDecoder(cv.Pgns[n].Rw)
	for {
		var m *Message
		go func() {
			if err := dec.Decode(m); err == io.EOF {
				decoded <- false
				return
			} else if err != nil {
				Logger.Println(err)
				decoded <- false
				return
			}
			decoded <- true
		}()

		select {
		case st := <-decoded:
			if !st {
				// quit if UI plugin disconnect
				if cv.Pgns[n].Name == "main" {
					close(cv.Quit)
				}
				return
			}
		case <-cv.Quit:
			return
		}

		if m.Domain == "Nagome" {
			switch m.Func {

			case FuncQueryBroad:
				switch m.Command {

				case CommQueryBroadConnect:
					var ct CtQueryBroadConnect
					if err := json.Unmarshal(m.Content, &ct); err != nil {
						Logger.Println("error in content:", err)
						continue
					}

					brdRg := regexp.MustCompile("(lv|co)\\d+")
					broadMch := brdRg.FindString(ct.BroadID)
					if broadMch == "" {
						Logger.Println("invalid BroadID")
						continue
					}

					cv.Lw = &nicolive.LiveWaku{Account: cv.Ac, BroadID: broadMch}

					nicoerr := cv.Lw.FetchInformation()
					if nicoerr != nil {
						Logger.Println(nicoerr)
						continue
					}

					eventReceiver := &commentEventEmit{cv: cv}
					cv.Cmm = nicolive.NewCommentConnection(cv.Lw, eventReceiver)
					nicoerr = cv.Cmm.Connect()
					if nicoerr != nil {
						Logger.Println(nicoerr)
						continue
					}
					defer cv.Cmm.Disconnect()

				case CommQueryBroadSendComment:
					var ct CtQueryBroadSendComment
					if err := json.Unmarshal(m.Content, &ct); err != nil {
						Logger.Println("error in content:", err)
						continue
					}
					cv.Cmm.SendComment(ct.Text, ct.Iyayo)

				default:
					Logger.Println("invalid Command in received message")
				}

			case FuncQueryAccount:
				switch m.Command {
				case CommQueryAccountLogin:
					err := cv.Ac.Login()
					if err != nil {
						Logger.Fatalln(err)
						continue
					}
					Logger.Println("logged in")

				case CommQueryAccountSave:
					cv.Ac.Save(filepath.Join(App.SavePath, "userData.yml"))

				case CommQueryAccountLoad:
					cv.Ac.Load(filepath.Join(App.SavePath, "userData.yml"))

				default:
					Logger.Println("invalid Command in received message")
				}

			case FuncComment:
				cv.Evch <- m

			default:
				Logger.Println("invalid Func in received message")
			}
		} else {
			cv.Evch <- m
		}
	}
}

func (cv *commentViewer) sendPluginEvent(i int, wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		select {
		case mes := <-cv.Evch:
			jmes, _ := json.Marshal(mes)
			for _, plug := range cv.Pgns {
				if plug.depend(mes.Domain) {
					Logger.Println(i)
					_, err := fmt.Fprintf(plug.Rw.Writer, "%d %s\n", i, jmes)
					if err != nil {
						Logger.Println(err)
					}
				}
			}
		case <-cv.Quit:
			return
		}
	}
}
