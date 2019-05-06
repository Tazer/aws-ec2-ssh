package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/helloyi/go-sshclient"
	"github.com/manifoldco/promptui"
	"golang.org/x/crypto/ssh"
)

var (
	user   string
	passwd string
	prikey string
)

func main() {
	servers := []Server{}
	flag.StringVar(&user, "user", "ec2-user", "The user of login")
	flag.StringVar(&passwd, "passwd", "yourpasswd", "The passwd of user")
	flag.StringVar(&prikey, "privatekey", "/.ssh/id_rsa", "The privatekey of user")

	flag.Parse()

	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String("eu-west-1"),
	}))

	ec2Svc := ec2.New(sess)

	result, err := ec2Svc.DescribeInstances(&ec2.DescribeInstancesInput{})
	if err != nil {
		log.Fatalf("Err: %v", err)
	}

	for _, reservation := range result.Reservations {
		for _, instance := range reservation.Instances {
			name := ""
			for _, tag := range instance.Tags {
				if *tag.Key == "Name" {
					name = *tag.Value
				}
			}
			servers = append(servers, Server{
				Name:      name,
				ID:        *instance.InstanceId,
				PrivateIP: *instance.PrivateIpAddress,
			})
		}
	}

	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}?",
		Active:   "\U0001F336 {{ .Name | cyan }} ({{ .ID | red }})",
		Inactive: "  {{ .Name | cyan }} ({{ .ID | red }})",
		Selected: "\U0001F336 {{ .Name | red | cyan }}",
		Details: `
--------- Servers ----------
{{ "Name:" | faint }}	{{ .Name }}
{{ "ID:" | faint }}	{{ .ID }}
{{ "Private IP:" | faint }}	{{ .PrivateIP }}`,
	}

	searcher := func(input string, index int) bool {
		server := servers[index]
		name := strings.Replace(strings.ToLower(server.Name), " ", "", -1)
		input = strings.Replace(strings.ToLower(input), " ", "", -1)

		return strings.Contains(name, input)
	}

	prompt := promptui.Select{
		Label:     "Select Server",
		Items:     servers,
		Templates: templates,
		Size:      20,
		Searcher:  searcher,
	}

	index, _, err := prompt.Run()

	if err != nil {
		fmt.Printf("Prompt failed %v\n", err)
		return
	}

	selectedServer := servers[index]

	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			PublicKeyFile(prikey),
			// ssh.Password(passwd),
		},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
	}
	client, err := sshclient.Dial("tcp", fmt.Sprintf("%s:22", selectedServer.PrivateIP), config)
	if err != nil {
		log.Fatalf("Err: %v", err)
	}
	defer client.Close()

	// default terminal
	// if err := client.Terminal(nil).Start(); err != nil {
	// 	log.Fatalf("Err: %v", err)
	// }

	// with a terminal config
	termConfig := &sshclient.TerminalConfig{
		Term:   "xterm",
		Weight: 80,
		Modes: ssh.TerminalModes{
			ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
			ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud

		},
	}
	if err := client.Terminal(termConfig).Start(); err != nil {
		log.Fatalf("Err: %v", err)
	}
}

func PublicKeyFile(file string) ssh.AuthMethod {

	home, err := os.UserHomeDir()

	if err != nil {
		log.Fatalf("Get home: %s err: %v", file, err)
	}

	buffer, err := ioutil.ReadFile(home + file)
	if err != nil {
		log.Fatalf("Couldnt read home: %s file: %s err: %v", home, file, err)
	}

	key, err := ssh.ParsePrivateKey(buffer)
	if err != nil {
		log.Fatalf("Couldnt ParsePrivateKey file: %s err: %v", file, err)
	}
	return ssh.PublicKeys(key)
}

type Server struct {
	Name      string
	ID        string
	PrivateIP string
	PublicIP  string
}

func (srv *Server) String() string {
	return fmt.Sprintf("%s (%s)", srv.Name, srv.ID)
}
