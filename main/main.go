package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"pavolmarko/hydra-srv/pkg/hydra"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var port = flag.Int("port", 80, "port to listen on")
var plainHttp = flag.Bool("plain-http", false, "if true, don't use https")
var certFile = flag.String("cert-file", "", "path to the server certificate")
var keyFile = flag.String("key-file", "", "path to the server private key")
var knownUsers = flag.String("known-users-file", "", "path to the file with known users")

func main() {
	flag.Parse()

	if *knownUsers == "" {
		fmt.Printf("Need --known-users-file\n")
		return
	}

	users, err := parseKnownUsers(*knownUsers)
	if err != nil {
		fmt.Printf("Error parsing %s: %v", *knownUsers, err)
		return
	}

	simHydra := &hydra.Sim{}
	simHydra.Start()

	http.HandleFunc("/ctl/{env}/{cmd}", func(w http.ResponseWriter, r *http.Request) {
		handleCmd(w, r, users, simHydra)
	})

	addr := fmt.Sprintf(":%d", *port)
	if *plainHttp {
		fmt.Println("Listening plain HTTP")
		if err := http.ListenAndServe(addr, nil); err != nil {
			fmt.Println("Error starting server:", err)
			return
		}
	} else {
		fmt.Println("Listening HTTPS")
		if err := http.ListenAndServeTLS(addr, *certFile, *keyFile, nil); err != nil {
			fmt.Println("Error starting server:", err)
			return
		}
	}
}

func parseKnownUsers(knownUsersFilePath string) (map[string]struct{}, error) {
	file, err := os.Open(knownUsersFilePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	res := map[string]struct{}{}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "//") {
			continue
		}

		res[line] = struct{}{}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return res, nil
}

func handleCmd(w http.ResponseWriter, r *http.Request, users map[string]struct{}, simHydra hydra.Inst) {
	auth := r.Header["Authorization"]
	if len(auth) == 0 || len(auth) > 1 {
		respondError(w, status.Errorf(codes.Unauthenticated, "need Authorization header (exactly 1)"))
		return
	}

	if !strings.HasPrefix(auth[0], "Bearer ") {
		respondError(w, status.Errorf(codes.Unauthenticated, "need Authorization: Bearer ... header"))
		return
	}

	bearer := auth[0][len("Bearer "):]
	if _, ok := users[bearer]; !ok {
		respondError(w, status.Errorf(codes.PermissionDenied, "auth failed"))
		return
	}

	var inst hydra.Inst

	env := r.PathValue("env")
	if env == "sim" {
		inst = simHydra
	} else {
		respondError(w, status.Errorf(codes.Unimplemented, "only simulated environment is available at the moment"))
		return
	}

	var resp any
	var err error

	cmd := r.PathValue("cmd")
	fmt.Printf("%s: %s %s\n", time.Now().Format(time.RFC3339), r.Method, r.URL.Path)

	if r.Method == "GET" {
		switch cmd {
		case "status":
			resp, err = inst.Status()
		default:
			err = status.Errorf(codes.Unimplemented, "unknown command GET %s", cmd)
		}
	} else if r.Method == "POST" {
		switch cmd {
		case "open":
			clientTime, err := readClientTime(r)
			if err != nil {
				respondError(w, err)
				return
			}

			resp, err = inst.Open(clientTime)

		case "close":
			clientTime, err := readClientTime(r)
			if err != nil {
				respondError(w, err)
				return
			}

			resp, err = inst.Close(clientTime)

		case "open-to-end":
			clientTime, err := readClientTime(r)
			if err != nil {
				respondError(w, err)
				return
			}

			resp, err = inst.OpenToEnd(clientTime)

		case "close-to-end":
			clientTime, err := readClientTime(r)
			if err != nil {
				respondError(w, err)
				return
			}

			resp, err = inst.CloseToEnd(clientTime)

		case "stop":
			clientTime, err := readClientTime(r)
			if err != nil {
				respondError(w, err)
				return
			}

			resp, err = inst.Stop(clientTime)

		case "sim-error":
			inst.SimError(true)
			resp = "ok, configured error"

		case "sim-no-error":
			inst.SimError(false)
			resp = "ok, configured no error"

		default:
			err = status.Errorf(codes.Unimplemented, "unknown command POST %s", cmd)
		}
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if err != nil {
		respondError(w, err)
		return
	}

	marshalled, err := json.Marshal(resp)
	if err != nil {
		respondError(w, err)
		return
	}

	fmt.Printf("  %d %s\n", http.StatusOK, string(marshalled))

	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(marshalled)))
	w.WriteHeader(http.StatusOK)
	w.Write(marshalled)
}

func readClientTime(r *http.Request) (time.Time, error) {
	var reqWithTime struct {
		RawTime string `json:"time"`
	}

	d := json.NewDecoder(r.Body)
	if err := d.Decode(&reqWithTime); err != nil {
		return time.Time{}, status.Errorf(codes.InvalidArgument, "can not parse request as JSON: %v", err)
	}

	parsed, err := time.Parse(time.RFC3339, reqWithTime.RawTime)
	if err != nil {
		return time.Time{}, status.Errorf(codes.InvalidArgument, "can not parse request-given time '%s' as RFC3339: %v", reqWithTime.RawTime, err)
	}

	return parsed, nil
}

func respondError(w http.ResponseWriter, err error) {
	httpStatus := 0
	httpBody := ""

	code := status.Code(err)
	if code == codes.Unknown {
		fmt.Printf("unknown err: %s\n", err.Error())
		httpStatus = http.StatusInternalServerError
	} else {
		httpStatus = toHttpStatus(code)
		httpBody = err.Error()
	}

	fmt.Printf("  %d %s\n", httpStatus, httpBody)

	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(httpBody)))
	w.WriteHeader(httpStatus)

	if len(httpBody) != 0 {
		w.Write([]byte(httpBody))
	}
}

func toHttpStatus(code codes.Code) int {
	switch code {
	case codes.Internal:
		return http.StatusBadRequest
	case codes.Unauthenticated:
		return http.StatusUnauthorized
	case codes.PermissionDenied:
		return http.StatusForbidden
	case codes.Unimplemented:
		return http.StatusNotFound
	case codes.Unavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}
