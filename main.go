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

func collectPods(ui *UIex){
	ui.Logger.Error("Getting pods")
	pods, err := ui.Kubik.GetPods()
	if err != nil {
		panic(err)
	}

	ui.Logger.Error("Updating podList")
	for i := range pods {
		name := PodToKey(pods[i])

		ui.Logger.Errorf("Adding item %s", name)
		ui.PodList.AddItem(
			name,
			string(pods[i].Status.Phase),
			0,
			func(){
				ui.APP.SetFocus(ui.CmdInput)
			})
	}
	ui.APP.Draw()
	ui.Logger.Error("podList updated")
}

type UIex struct {
	APP *tview.Application
	MainBox *tview.Flex
	PodList *tview.List
	Log *tview.TextView
	CmdInput *tview.InputField

	Logger *logrus.Logger

	Kubik  Kubik
	PodMap map[string]coreV1.Pod
}

func buildTviewApp(ui *UIex){
	ui.Logger.Error("UI generating")
	APP := tview.NewApplication()

	podList := tview.NewList()
	podList.Box.SetBorder(true)

	logView := tview.NewTextView()
	logView.
		SetDynamicColors(true).
		SetRegions(true).
		SetWordWrap(true).
		SetChangedFunc(func() {
			APP.Draw()
		}).SetBorder(true)

	inputField := tview.NewInputField().
		SetLabel("#: ").
		SetDoneFunc(func(key tcell.Key) {
			CmdInputHandler(key, *ui)
	})
	inputField.SetBorder(true)
	inputField.GetFocusable()

	rightCol := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(logView, 0, 1, false).
		AddItem(inputField, 3, 1, true)

	flex := tview.NewFlex().
		AddItem(podList, 40, 1, true).
		AddItem(rightCol, 0, 1, true)

	ui.APP = APP
	ui.MainBox = flex
	ui.PodList = podList
	ui.Log = logView
	ui.CmdInput = inputField

	ui.Logger.Error("UI generated")
}

func CmdInputHandler(key tcell.Key, ui UIex){
	if key == 9 {
		ui.APP.SetFocus(ui.PodList)
	} else if key == 13 {
		i := ui.PodList.GetCurrentItem()
		podName, _ := ui.PodList.GetItemText(i)
		if len(podName) > 0 {
			nsn := strings.Split(podName, "/")
			if len(nsn) != 2 {
				panic(fmt.Sprintf("Incorrect name from UI %s", podName))
			}

			go func (){
				out, err := ui.Kubik.Run(
					ui.Kubik.GetPod(nsn[0], nsn[1]),
					strings.Split(ui.CmdInput.GetText(), " "))
				if err != nil {
					msg := fmt.Sprintf("%s #> \n%s", podName, err.Error())
					ui.Log.Write([]byte(msg))
				} else {
					msg := fmt.Sprintf("%s #> \n%s", podName, string(out))
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

	ui.Logger.Error("Create kubik")
	ui.Kubik = NewKubik(ui, "")
	ui.Logger.Error("Kubik created")

	buildTviewApp(&ui)

	go collectPods(&ui)
	if err := ui.APP.SetRoot(ui.MainBox, true).Run(); err != nil {
		panic(err)
	}
}
