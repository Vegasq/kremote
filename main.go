package main

import (
	"fmt"
	"github.com/gdamore/tcell"
	"github.com/sirupsen/logrus"
	coreV1 "k8s.io/api/core/v1"
	"github.com/rivo/tview"
	"os"
	"strings"
)

func addPodsToPodList(ui *UIex, pods []coreV1.Pod){
	for i := range pods {
		name := PodToKey(pods[i])

		ui.Logger.Errorf("Adding item %s", name)
		ui.PodList.AddItem(
			name,
			string(pods[i].Status.Phase),
			0,
			func(){
				ui.APP.SetFocus(ui.CmdInput)
			},
		)
	}
}

func collectPods(ui *UIex){
	ui.Logger.Error("Getting pods")
	pods, err := ui.Kubik.GetPods()
	if err != nil {
		panic(err)
	}

	ui.Logger.Error("Updating podList")
	addPodsToPodList(ui, pods)

	ui.APP.Draw()
	ui.Logger.Error("podList updated")
}

type UIex struct {
	APP *tview.Application
	MainBox *tview.Flex
	PodList *tview.List
	Log *tview.TextView
	CmdInput *tview.InputField
	Filter *tview.InputField
	EnvList *tview.DropDown

	Logger *logrus.Logger

	Kubik  Kubik
	PodMap map[string]coreV1.Pod
}

func buildTviewApp(ui *UIex){
	ui.Logger.Error("UI generating")
	APP := tview.NewApplication()

	envSelector := tview.NewDropDown().
		SetLabel("Select kubeconfig: ").
		SetOptions([]string{"/Users/myakovliev/kube_52b/admin/kubeconfig2.yaml",
							"/Users/myakovliev/kube_52b/admin/kubeconfig.yaml"}, nil)
	envSelector.SetBorder(true)
	envSelector.SetSelectedFunc(func (t string, i int) {
		go func(){
			ui.Log.Write([]byte(fmt.Sprintf("Loading %s\n", t)))
			ui.Logger.Errorf("Create kubik for %s\n", t)
			ui.Kubik = NewKubik(*ui, t)
			ui.Logger.Errorf("Kubik created for %s\n", t)
			ui.APP.SetFocus(ui.PodList)
			collectPods(ui)
		}()
	})
	envSelector.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
		if e.Key() == 9 {
			ui.APP.SetFocus(ui.Filter)
		}
		return e
	})

	podList := tview.NewList()
	podList.Box.SetBorder(true)
	podList.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
		if e.Key() == 9 {
			ui.APP.SetFocus(ui.Log)
		}
		return e
	})

	logView := tview.NewTextView()
	logView.
		SetDynamicColors(true).
		SetRegions(true).
		SetWordWrap(true).
		SetChangedFunc(func() {
			APP.Draw()
		}).SetBorder(true).
		SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
			if e.Key() == 9 {
				ui.APP.SetFocus(ui.EnvList)
			}
			//ui.Log.Write([]byte(fmt.Sprintf("%d", e.Key())))
			return e
		})
	logView.SetRegions(true)

	inputField := tview.NewInputField().
		SetLabel("#: ").
		SetDoneFunc(func(key tcell.Key) {
			CmdInputHandler(key, *ui)
	})
	inputField.SetBorder(true)

	filter := tview.NewInputField().
		SetLabel("filter: ").
		SetDoneFunc(func(key tcell.Key) {
			if key != 13 {
				return
			}

			ui.PodList.Clear()

			pods, err := ui.Kubik.GetPods()
			if err != nil {
				panic(err)
			}
			txt := ui.Filter.GetText()
			filtered := []coreV1.Pod{}
			for i := range pods {
				if strings.Contains(pods[i].Name, txt) || strings.Contains(pods[i].Namespace, txt) {
					filtered = append(filtered, pods[i])
				}
			}

			addPodsToPodList(ui, filtered)
			ui.APP.SetFocus(ui.PodList)
		})
	filter.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
			if e.Key() == 9 {
				ui.APP.SetFocus(ui.PodList)
			}
			return e
		})
	filter.SetBorder(true)

	//inputField.GetFocusable()
	leftCol := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(filter, 3, 1, false).
		AddItem(podList, 0, 1, true)

	rightCol := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(envSelector, 3, 1, true).
		AddItem(logView, 0, 1, false).
		AddItem(inputField, 3, 1, false)

	flex := tview.NewFlex().
		AddItem(leftCol, 40, 10, false).
		AddItem(rightCol, 0, 1, false)

	ui.APP = APP
	ui.MainBox = flex
	ui.PodList = podList
	ui.Log = logView
	ui.CmdInput = inputField
	ui.Filter = filter
	ui.EnvList = envSelector

	ui.Logger.Error("UI generated")
}

func CmdInputHandler(key tcell.Key, ui UIex){
	if key == 9 {
		ui.APP.SetFocus(ui.Log)
		//ui.APP.SetFocus(ui.PodList)
	} else if key == 13 {
		i := ui.PodList.GetCurrentItem()
		podName, _ := ui.PodList.GetItemText(i)
		if len(podName) > 0 {
			nsn := strings.Split(podName, "/")
			if len(nsn) != 2 {
				panic(fmt.Sprintf("Incorrect name from UI %s", podName))
			}

			go func (){
				cmd := strings.Split(ui.CmdInput.GetText(), " ")
				ui.CmdInput.SetText("")

				msg := fmt.Sprintf("[\"rh\"]%s> Dispatch command: %s\n", podName, strings.Join(cmd, " "))
				ui.Log.Write([]byte(msg))

				out, err := ui.Kubik.Run(ui.Kubik.GetPod(nsn[0], nsn[1]), cmd)
				if err != nil {
					msg := fmt.Sprintf("[\"rh\"]%s #> \n%s", podName, err.Error())
					ui.Log.Write([]byte(msg))
				} else {
					msg := fmt.Sprintf("[\"rh\"]%s #> \n%s", podName, string(out))
					ui.Log.Write([]byte(msg))
				}
			}()
		}
	}
}

func getLog() *logrus.Logger {
	l := logrus.New()
	f, err := os.OpenFile("log2.log", os.O_WRONLY | os.O_APPEND | os.O_CREATE, 0644)

	if err != nil {
		// Cannot open log file. Logging to stderr
		fmt.Println(err)
	} else {
		l.SetOutput(f)
	}

	return l
}

func main(){
	ui := UIex{}
	ui.Logger = getLog()

	//ui.Logger.Error("Create kubik")
	//ui.Kubik = NewKubik(ui, "")
	//ui.Logger.Error("Kubik created")

	buildTviewApp(&ui)

	//go collectPods(&ui)
	//ui.APP.SetFocus(ui.EnvList)
	if err := ui.APP.SetRoot(ui.MainBox, true).SetFocus(ui.EnvList).Run(); err != nil {
		panic(err)
	}
}
