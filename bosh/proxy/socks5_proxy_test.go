package proxy_test

import (
	"bufio"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	goproxy "golang.org/x/net/proxy"

	"github.com/cloudfoundry/bosh-deployment-resource/bosh/proxy"
	"github.com/cloudfoundry/bosh-deployment-resource/bosh/proxy/proxyfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Socks5Proxy", func() {
	var (
		socks5Proxy   *proxy.Socks5Proxy
		hostKeyGetter *proxyfakes.FakeKeyGetter

		sshServerURL       string
		httpServerHostPort string
		httpServer         *httptest.Server
		proxyError         chan error
	)

	BeforeEach(func() {
		httpServer = httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			rw.WriteHeader(http.StatusOK)
		}))
		httpServerHostPort = strings.Split(httpServer.URL, "http://")[1]

		sshServerURL = startSSHServer(httpServerHostPort)

		signer, err := ssh.ParsePrivateKey([]byte(sshPrivateKey))
		Expect(err).NotTo(HaveOccurred())

		hostKeyGetter = &proxyfakes.FakeKeyGetter{}
		hostKeyGetter.GetReturns(signer.PublicKey(), nil)

		proxyError = make(chan error)
		socks5Proxy = proxy.NewSocks5Proxy(hostKeyGetter)
	})

	AfterEach(func() {
		proxy.ResetNetListen()
	})

	Describe("Start", func() {
		It("starts a proxy to the jumpbox", func() {
			err := socks5Proxy.Start(sshPrivateKey, sshServerURL)
			Expect(err).NotTo(HaveOccurred())

			// Wait for socks5 proxy to start
			time.Sleep(1 * time.Second)

			socks5Addr, err := socks5Proxy.Addr()
			Expect(err).NotTo(HaveOccurred())

			socks5Client, err := goproxy.SOCKS5("tcp", socks5Addr, nil, goproxy.Direct)
			Expect(err).NotTo(HaveOccurred())

			Expect(hostKeyGetter.GetCallCount()).To(Equal(1))
			key, url := hostKeyGetter.GetArgsForCall(0)
			Expect(key).To(Equal(sshPrivateKey))
			Expect(url).To(Equal(sshServerURL))

			conn, err := socks5Client.Dial("tcp", httpServerHostPort)
			Expect(err).NotTo(HaveOccurred())

			_, err = conn.Write([]byte("GET / HTTP/1.0\r\n\r\n"))
			Expect(err).NotTo(HaveOccurred())
			defer conn.Close()

			status, err := bufio.NewReader(conn).ReadString('\n')
			Expect(status).To(Equal("HTTP/1.0 200 OK\r\n"))
		})

		Context("when starting the proxy a second time", func() {
			It("no-ops on the second run", func() {
				err := socks5Proxy.Start(sshPrivateKey, sshServerURL)
				Expect(err).NotTo(HaveOccurred())

				// Wait for socks5 proxy to start
				time.Sleep(1 * time.Second)

				err = socks5Proxy.Start(sshPrivateKey, sshServerURL)
				Expect(err).NotTo(HaveOccurred())

				socks5Addr, err := socks5Proxy.Addr()
				Expect(err).NotTo(HaveOccurred())

				socks5Client, err := goproxy.SOCKS5("tcp", socks5Addr, nil, goproxy.Direct)
				Expect(err).NotTo(HaveOccurred())

				conn, err := socks5Client.Dial("tcp", httpServerHostPort)
				Expect(err).NotTo(HaveOccurred())

				_, err = conn.Write([]byte("GET / HTTP/1.0\r\n\r\n"))
				Expect(err).NotTo(HaveOccurred())
				defer conn.Close()

				status, err := bufio.NewReader(conn).ReadString('\n')
				Expect(status).To(Equal("HTTP/1.0 200 OK\r\n"))
			})
		})

		Context("failure cases", func() {
			Context("when it cannot parse the private key", func() {
				It("returns an error", func() {
					err := socks5Proxy.Start("some-bad-private-key", sshServerURL)
					Expect(err).To(MatchError("parse private key: ssh: no key found"))
				})
			})

			Context("when it cannot get the host key", func() {
				BeforeEach(func() {
					hostKeyGetter.GetReturns(nil, errors.New("banana"))
				})

				It("returns an error", func() {
					err := socks5Proxy.Start(sshPrivateKey, sshServerURL)
					Expect(err).To(MatchError("get host key: banana"))
				})
			})

			Context("when it cannot dial the jumpbox url", func() {
				It("returns an error", func() {
					err := socks5Proxy.Start(sshPrivateKey, "some-bad-url")
					Expect(err).To(MatchError("ssh dial: dial tcp: address some-bad-url: missing port in address"))
				})
			})

			Context("when opening the port fails", func() {
				It("returns an error", func() {
					proxy.SetNetListen(func(string, string) (net.Listener, error) {
						return nil, errors.New("coconut")
					})

					err := socks5Proxy.Start(sshPrivateKey, sshServerURL)
					Expect(err).To(MatchError("open port: coconut"))
				})
			})
		})
	})

	Describe("Addr", func() {
		Context("when the proxy has been started", func() {
			BeforeEach(func() {
				err := socks5Proxy.Start(sshPrivateKey, sshServerURL)
				Expect(err).NotTo(HaveOccurred())
				// Wait for socks5 proxy to start
				time.Sleep(1 * time.Second)
			})

			It("returns a valid address of the socks5 proxy", func() {
				addr, err := socks5Proxy.Addr()
				Expect(err).NotTo(HaveOccurred())
				Expect(addr).To(MatchRegexp("127.0.0.1:\\d+"))
			})
		})

		Context("when no proxy has been started", func() {
			It("returns an error", func() {
				_, err := socks5Proxy.Addr()
				Expect(err).To(MatchError("socks5 proxy is not running"))
			})
		})
	})
})