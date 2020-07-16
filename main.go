// https://github.com/slack-go/slack
// https://guzalexander.com/2017/09/15/cowsay-slack-command.html

package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"encoding/json"

	"github.com/slack-go/slack"
)

var (
	port  string = "80"
	token string
)

func init() {
	token = os.Getenv("VERIFICATION_TOKEN")
	if "" == token {
		panic("VERIFICATION_TOKEN is not set!")
	}

	if "" != os.Getenv("PORT") {
		port = os.Getenv("PORT")
	}
}

func botHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	if token != r.FormValue("token") {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	r.ParseForm()

	responseURL := r.Form["response_url"][0]

	// field info here: https://api.slack.com/interactivity/slash-commands

	command := r.Form["command"][0]
	userid := r.Form["user_id"][0]
	username := r.Form["user_name"][0]
	channelid := r.Form["channel_id"][0]
	channelname := r.Form["channel_name"][0]
	text := r.Form["text"][0]
	textArray := strings.Split(text, " ")

	fmt.Println("command: ", command, "\nuser_id: ", userid, "\nuser_name: ", username, "\nchannel_id", channelid, "\nchannel_name:", channelname, "\ntext: ", text)
	fmt.Printf("textArray:'%s'\n", textArray)

	botReply := "Processing your request, please standby ⏳"
	jsonResp, _ := json.Marshal(struct {
		Type string `json:"response_type"`
		Text string `json:"text"`
	}{
		Type: "in_channel",
		Text: fmt.Sprintf("```%s```", botReply),
	})

	w.Header().Add("Content-Type", "application/json")
	fmt.Fprintf(w, string(jsonResp))

	go handleCommand(responseURL, command, userid, textArray)
}

func handleCommand(responseURL, command, userid string, textArray []string) {
	var botReply string
	switch command {
	case "/brrr":
		botReply = brrr(userid, textArray)
	case "/send":
		botReply = send(userid, textArray)
	case "/balance":
		botReply = balance(userid, textArray)
	default:
		botReply = fmt.Sprintf("Sorry I don't understand that command %s.", command)
	}

	jsonResp, _ := json.Marshal(struct {
		Type string `json:"response_type"`
		Text string `json:"text"`
	}{
		Type: "in_channel",
		Text: fmt.Sprintf("```%s```", botReply),
	})

	fmt.Println("responseURL", responseURL)
	resp, err := http.Post(responseURL, "application/json", bytes.NewBuffer(jsonResp))
	if err != nil {
		fmt.Println("err", err)
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("err", err)
	}

	fmt.Println(string(body))

}
func getUserID(userID string) (string, string, error) {
	api := slack.New(os.Getenv("API_TOKEN"))
	// fmt.Println("userID", userID)
	// fmt.Println("usernameOriginal", usernameOriginal)
	// username := strings.Split(usernameOriginal, "|")[1]
	// username = username[:len(username)-1]
	// fmt.Println("username", username)

	user, err := api.GetUserInfo(userID)
	if err != nil {
		fmt.Printf("%s\n", err)
		return userID, "", err
	}

	username := user.Profile.DisplayNameNormalized
	return user.ID, username, nil
	// fmt.Printf("ID: %s, Fullname: %s, Email: %s\n", user.ID, user.Profile.RealName, user.Profile.Email)
}

const ShellToUse = "bash"

func Shellout(command string) (error, string, string) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.Command(ShellToUse, "-c", command)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return err, stdout.String(), stderr.String()
}

func createNewUserAccount(user, username string) error {
	username = strings.ReplaceAll(username, " ", "_")
	err, out, errout := Shellout(fmt.Sprintf("pooltoycli tx pooltoy create-user $(pooltoycli keys show %s -a) false %s %s --from alice -y", user, username, user))
	fmt.Println("err", err)
	fmt.Println("out", out)
	fmt.Println("errout", errout)
	time.Sleep(5 * time.Second)
	return nil
}
func createNewUserKey(user, username string) error {
	err, _, errout := Shellout(fmt.Sprintf("pooltoycli keys add %s", user))

	if err == nil {
		path, err := os.Getwd()
		if err != nil {
			fmt.Println(err)
			return err
		}
		filename := fmt.Sprintf("%s/keys/%s.json", path, user)
		fmt.Printf("New user %s created and backup saved at %s\n", username, filename)
		d1 := []byte(errout)
		err = ioutil.WriteFile(filename, d1, 0644)
		if err != nil {
			fmt.Println(err)
		}
	}
	return createNewUserAccount(user, username)
}
func confirmUser(user, username string) error {
	err, out, errout := Shellout(fmt.Sprintf("pooltoycli keys show %s", user))
	fmt.Println("err", err)
	fmt.Println("out", out)
	fmt.Println("errout", errout)
	if err != nil {
		// there's an error, find out if it's just that the key doesn't exist
		if errout == "ERROR: The specified item could not be found in the keyring\n" {
			return createNewUserKey(user, username)
		} else {
			fmt.Printf("'%s' didn't match\n", errout)
			return err
		}
	}
	return nil
}
func brrr(userid string, text []string) string {

	if len(text) != 2 {
		return "Sorry, I don't understand that command. Please follow the format '/brrr [recipient] [emoji]' where emoji is part of the basic emoji list outlined here: https://unicode.org/Public/emoji/5.0/emoji-test.txt"
	}

	// confirm sender user id key exists
	// if not create key
	// if not create account
	senderID, senderUsername, err := getUserID(userid)
	if err != nil {
		return fmt.Sprintf("ERROR: %s (%s)", err.Error(), senderID)
	}
	err = confirmUser(senderID, senderUsername)
	if err != nil {
		return fmt.Sprintf("ERROR: %s (%s)", err.Error(), userid)
	}

	// confirm recipientID key exists
	// if not create key
	// if not create account

	recipientID, recipientUsername, err := getUserID(strings.Split(text[0], "|")[0][2:])
	if err != nil {
		return fmt.Sprintf("ERROR: %s (%s)", err.Error(), recipientID)
	}
	err = confirmUser(recipientID, recipientUsername)
	if err != nil {
		return fmt.Sprintf("ERROR: %s (%s)", err.Error(), userid)
	}

	emoji := text[1]
	if emojiCodeMap[emoji] != "" {
		emoji = emojiCodeMap[emoji]
	}
	command := fmt.Sprintf("pooltoycli tx faucet mintfor $(pooltoycli keys show %s -a) %s --from %s -y", recipientID, emoji, senderID)
	fmt.Printf("Try command '%s\n", command)

	// create the CLI command for faucet from userid to recipientID
	err, out, errout := Shellout(command)

	fmt.Println("err", err)
	fmt.Println("out", out)
	fmt.Println("errout", errout)

	// parse various responses
	if err != nil {
		return err.Error()
	}

	type TxResult struct {
		Height string
		Txhash string
		RawLog string
	}

	var txResult TxResult
	json.Unmarshal([]byte(out), &txResult)

	fmt.Println("txResult.Txhash", txResult.Txhash)
	// wait until the tx is processed
	time.Sleep(5 * time.Second)

	query := fmt.Sprintf("pooltoycli q tx %s", txResult.Txhash)
	err, out, errout = Shellout(query)

	fmt.Println("err", err)
	fmt.Println("out", out)
	fmt.Println("errout", errout)

	var qResult map[string]interface{}
	json.Unmarshal([]byte(out), &qResult)

	fmt.Println("qResult", qResult)
	fmt.Println("qResult[\"codespace\"]", qResult["codespace"])

	// codespace is part of an error log
	if qResult["codespace"] != nil {
		return "Sorry, you can only send an emoji once a day. Please try again tomorrow 📆"
	}

	return fmt.Sprintf("Success! You sent %s a %s. Check their balance like: /balance @%s", recipientUsername, emoji, recipientUsername)
}
func send(userid string, text []string) string {
	return "send"
}
func balance(userid string, text []string) string {
	return "balance"
}

func main() {
	http.HandleFunc("/", botHandler)
	log.Fatalln(http.ListenAndServe(":"+port, nil))
}
