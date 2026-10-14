package credentials

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

const (
	envSocket = "FOCAGIT_ASKPASS_SOCKET"
	envToken  = "FOCAGIT_ASKPASS_TOKEN"
	// helperFlag tells the binary git started it as a credential helper rather than as askpass.
	helperFlag   = "--credential"
	actionPrompt = "prompt"
)

// Request is one call from the askpass or credential-helper process.
type Request struct {
	Token  string            `json:"token"`
	Action string            `json:"action"`
	Prompt string            `json:"prompt,omitempty"`
	Fields map[string]string `json:"fields,omitempty"`
	// Parent is the process that asked, so a repeated passphrase prompt from one ssh means a wrong answer.
	Parent int `json:"parent,omitempty"`
}

// Response is the answer to print; OK false makes askpass fail, which git reports.
type Response struct {
	OK     bool              `json:"ok"`
	Answer string            `json:"answer,omitempty"`
	Fields map[string]string `json:"fields,omitempty"`
}

// HandleFunc answers a request; ctx ends when the caller goes away or the server closes.
type HandleFunc func(ctx context.Context, req Request) Response

// Server listens on a private unix socket for the processes git starts.
type Server struct {
	dir    string
	path   string
	token  string
	ln     net.Listener
	handle HandleFunc

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// Listen opens the socket in a directory only this user can enter.
func Listen(handle HandleFunc) (*Server, error) {
	base := os.Getenv("XDG_RUNTIME_DIR")
	if base == "" {
		base = os.TempDir()
	}
	dir, err := os.MkdirTemp(base, "focagit-")
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "askpass.sock")
	ln, err := net.Listen("unix", path)
	if err != nil {
		os.RemoveAll(dir)
		return nil, err
	}

	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		ln.Close()
		os.RemoveAll(dir)
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &Server{dir: dir, path: path, token: hex.EncodeToString(secret), ln: ln, handle: handle, ctx: ctx, cancel: cancel}
	s.wg.Add(1)
	go s.serve()
	return s, nil
}

// Env is what git needs to reach this server: exe is the binary that runs Main.
func (s *Server) Env(exe string) []string {
	// Appended after any config the environment already adds, so the user's own helpers are asked first.
	n, _ := strconv.Atoi(os.Getenv("GIT_CONFIG_COUNT"))
	return []string{
		"GIT_ASKPASS=" + exe,
		// ssh asks for key passphrases the same way, even when it has a terminal.
		"SSH_ASKPASS=" + exe,
		"SSH_ASKPASS_REQUIRE=force",
		envSocket + "=" + s.path,
		envToken + "=" + s.token,
		"GIT_CONFIG_COUNT=" + strconv.Itoa(n+1),
		"GIT_CONFIG_KEY_" + strconv.Itoa(n) + "=credential.helper",
		"GIT_CONFIG_VALUE_" + strconv.Itoa(n) + "=!" + shellQuote(exe) + " " + helperFlag,
	}
}

// Close stops listening, cancels waiting questions and removes the socket.
func (s *Server) Close() {
	s.cancel()
	s.ln.Close()
	s.wg.Wait()
	os.RemoveAll(s.dir)
}

func (s *Server) serve() {
	defer s.wg.Done()
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.answer(conn)
		}()
	}
}

// answer reads one request and writes one response; a question is dropped if the caller hangs up.
func (s *Server) answer(conn net.Conn) {
	defer conn.Close()
	r := bufio.NewReader(conn)
	line, err := r.ReadBytes('\n')
	if err != nil {
		return
	}
	var req Request
	if json.Unmarshal(line, &req) != nil || subtle.ConstantTimeCompare([]byte(req.Token), []byte(s.token)) != 1 {
		return
	}

	ctx, cancel := context.WithCancel(s.ctx)
	defer cancel()
	// The caller sends nothing more, so a finished read means it went away: git was killed.
	go func() {
		_, _ = r.ReadByte()
		cancel()
	}()
	// Closing the connection on shutdown unblocks that read.
	stop := context.AfterFunc(s.ctx, func() { conn.Close() })
	defer stop()

	out, _ := json.Marshal(s.handle(ctx, req))
	_, _ = conn.Write(append(out, '\n'))
}

// call sends one request to the server and waits for its response.
func call(path string, req Request) (Response, error) {
	conn, err := net.Dial("unix", path)
	if err != nil {
		return Response{}, err
	}
	defer conn.Close()
	b, _ := json.Marshal(req)
	if _, err := conn.Write(append(b, '\n')); err != nil {
		return Response{}, err
	}
	line, err := bufio.NewReader(conn).ReadBytes('\n')
	if err != nil {
		return Response{}, err
	}
	var resp Response
	if err := json.Unmarshal(line, &resp); err != nil {
		return Response{}, errors.New("bad response from focagit")
	}
	return resp, nil
}

// shellQuote wraps s in single quotes for the shell git runs the helper through.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
