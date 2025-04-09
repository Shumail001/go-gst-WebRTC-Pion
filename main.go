//package main
//
//import (
//	"encoding/base64"
//	"encoding/json"
//	"flag"
//	"fmt"
//	"io"
//	"net/http"
//	"strconv"
//
//	"github.com/go-gst/go-gst/gst"
//	"github.com/go-gst/go-gst/gst/app"
//	"github.com/pion/webrtc/v4"
//	"github.com/pion/webrtc/v4/pkg/media"
//)
//
//func main() {
//	audioSrc := flag.String("audio-src", "pulsesrc", "GStreamer audio src")
//	videoSrc := flag.String("video-src", "v4l2src", "GStreamer video src")
//	videoDevice := flag.String("video-device", "/dev/video0", "Video device to use")
//	port := flag.Int("port", 8080, "http server port")
//	bitrate := flag.Int("bitrate", 2000, "Video bitrate in kbps")
//	framerate := flag.String("framerate", "30/1", "Video framerate")
//	resolution := flag.String("resolution", "1280x720", "Video resolution (widthxheight)")
//	flag.Parse()
//
//	sdpChan := httpSDPServer(*port)
//
//	// Initialize GStreamer
//	gst.Init(nil)
//
//	// Prepare the configuration
//	config := webrtc.Configuration{
//		ICEServers: []webrtc.ICEServer{
//			{
//				URLs: []string{"stun:stun.l.google.com:19302"},
//			},
//		},
//	}
//
//	// Create a new RTCPeerConnection
//	peerConnection, err := webrtc.NewPeerConnection(config)
//	if err != nil {
//		panic(err)
//	}
//
//	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
//		fmt.Printf("Connection State has changed %s \n", connectionState.String())
//	})
//
//	// Create tracks
//	opusTrack, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: "audio/opus"}, "audio", "pion1")
//	if err != nil {
//		panic(err)
//	} else if _, err = peerConnection.AddTrack(opusTrack); err != nil {
//		panic(err)
//	}
//
//	h264Track, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: "video/h264"}, "video", "pion2")
//	if err != nil {
//		panic(err)
//	} else if _, err = peerConnection.AddTrack(h264Track); err != nil {
//		panic(err)
//	}
//
//	// Create and set offer
//	offer, err := peerConnection.CreateOffer(nil)
//	if err != nil {
//		panic(err)
//	}
//
//	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)
//	if err = peerConnection.SetLocalDescription(offer); err != nil {
//		panic(err)
//	}
//	<-gatherComplete
//
//	fmt.Println(encode(peerConnection.LocalDescription()))
//
//	// Wait for answer
//	answer := webrtc.SessionDescription{}
//	decode(<-sdpChan, &answer)
//	if err = peerConnection.SetRemoteDescription(answer); err != nil {
//		panic(err)
//	}
//
//	// Build pipelines with NVIDIA hardware acceleration
//	width, height := parseResolution(*resolution)
//	videoPipelineSrc := fmt.Sprintf(
//		"%s device=%s ! videoconvert ! videoscale ! video/x-raw,width=%d,height=%d,framerate=%s,format=NV12 ! queue",
//		*videoSrc, *videoDevice, width, height, *framerate)
//
//	audioPipelineSrc := *audioSrc + " ! audioconvert ! audioresample ! audio/x-raw,channels=1,rate=48000 ! queue"
//
//	// Start pipelines
//	pipelineForCodec("opus", []*webrtc.TrackLocalStaticSample{opusTrack}, audioPipelineSrc)
//	pipelineForCodec("h264-nvidia", []*webrtc.TrackLocalStaticSample{h264Track}, videoPipelineSrc, *bitrate)
//
//	select {}
//}
//
//func parseResolution(res string) (int, int) {
//	var width, height int
//	_, err := fmt.Sscanf(res, "%dx%d", &width, &height)
//	if err != nil {
//		return 1280, 720
//	}
//	return width, height
//}
//
//func pipelineForCodec(codecName string, tracks []*webrtc.TrackLocalStaticSample, pipelineSrc string, bitrate ...int) {
//	pipelineStr := "appsink name=appsink"
//	switch codecName {
//	case "h264-nvidia":
//		// NVIDIA NVENC H.264 encoder pipeline
//		br := 2000
//		if len(bitrate) > 0 {
//			br = bitrate[0]
//		}
//		pipelineStr = pipelineSrc + fmt.Sprintf(
//			` ! nvh264enc
//			  preset=low-latency-hq
//			  bitrate=%d
//			  rc-mode=cbr
//			  ! h264parse
//			  ! video/x-h264,stream-format=byte-stream
//			  ! %s`, br, pipelineStr)
//	case "opus":
//		pipelineStr = pipelineSrc + " ! opusenc ! " + pipelineStr
//	default:
//		panic("Unhandled codec " + codecName)
//	}
//
//	pipeline, err := gst.NewPipelineFromString(pipelineStr)
//	if err != nil {
//		panic(err)
//	}
//
//	if err = pipeline.SetState(gst.StatePlaying); err != nil {
//		panic(err)
//	}
//
//	appSink, err := pipeline.GetElementByName("appsink")
//	if err != nil {
//		panic(err)
//	}
//
//	app.SinkFromElement(appSink).SetCallbacks(&app.SinkCallbacks{
//		NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
//			sample := sink.PullSample()
//			if sample == nil {
//				return gst.FlowEOS
//			}
//
//			buffer := sample.GetBuffer()
//			if buffer == nil {
//				return gst.FlowError
//			}
//
//			samples := buffer.Map(gst.MapRead).Bytes()
//			defer buffer.Unmap()
//
//			for _, t := range tracks {
//				if err := t.WriteSample(media.Sample{
//					Data:     samples,
//					Duration: *buffer.Duration().AsDuration(),
//				}); err != nil {
//					panic(err)
//				}
//			}
//
//			return gst.FlowOK
//		},
//	})
//}
//
//func encode(obj *webrtc.SessionDescription) string {
//	b, err := json.Marshal(obj)
//	if err != nil {
//		panic(err)
//	}
//	return base64.StdEncoding.EncodeToString(b)
//}
//
//func decode(in string, obj *webrtc.SessionDescription) {
//	b, err := base64.StdEncoding.DecodeString(in)
//	if err != nil {
//		panic(err)
//	}
//	if err = json.Unmarshal(b, obj); err != nil {
//		panic(err)
//	}
//}
//
//func httpSDPServer(port int) chan string {
//	sdpChan := make(chan string)
//	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
//		body, _ := io.ReadAll(r.Body)
//		fmt.Fprintf(w, "done")
//		sdpChan <- string(body)
//	})
//
//	go func() {
//		panic(http.ListenAndServe(":"+strconv.Itoa(port), nil))
//	}()
//
//	return sdpChan
//}

// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
)

var (
	videoDevice string
	bitrate     int
	upgrader    = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
)

type Message struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

func main() {
	flag.StringVar(&videoDevice, "video-device", "/dev/video0", "Video device to use")
	flag.IntVar(&bitrate, "bitrate", 2000, "Video bitrate in kbps")
	port := flag.Int("port", 8080, "http server port")
	flag.Parse()

	http.HandleFunc("/ws", wsHandler)
	http.Handle("/", http.FileServer(http.Dir(".")))

	log.Printf("Server starting on port %d", *port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", *port), nil); err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("Upgrade failed: ", err)
		return
	}
	defer conn.Close()

	// Initialize GStreamer
	gst.Init(nil)

	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	}

	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		log.Print("PeerConnection failed: ", err)
		return
	}

	// Create tracks
	opusTrack, err := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: "audio/opus"}, "audio", "pion1")
	if err != nil {
		log.Print("Audio track failed: ", err)
		return
	}
	if _, err = peerConnection.AddTrack(opusTrack); err != nil {
		log.Print("Add audio track failed: ", err)
		return
	}

	h264Track, err := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: "video/h264"}, "video", "pion2")
	if err != nil {
		log.Print("Video track failed: ", err)
		return
	}
	sender, err := peerConnection.AddTrack(h264Track)
	if err != nil {
		log.Print("Add video track failed: ", err)
		return
	}

	log.Printf("Added video track, sender: %+v", sender)
	// Handle ICE candidates
	peerConnection.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c != nil {
			if err := conn.WriteJSON(Message{
				Type: "candidate",
				Data: string(c.ToJSON().Candidate),
			}); err != nil {
				log.Print("Write candidate failed: ", err)
			}
		}
	})

	peerConnection.OnICEConnectionStateChange(func(s webrtc.ICEConnectionState) {
		log.Printf("ICE Connection State changed: %s", s.String())
	})

	// Create and send offer
	offer, err := peerConnection.CreateOffer(nil)
	if err != nil {
		log.Print("CreateOffer failed: ", err)
		return
	}

	if err = peerConnection.SetLocalDescription(offer); err != nil {
		log.Print("SetLocalDescription failed: ", err)
		return
	}

	// Send offer to client
	if err = conn.WriteJSON(Message{
		Type: "offer",
		Data: offer.SDP,
	}); err != nil {
		log.Print("Write offer failed: ", err)
		return
	}

	// Handle incoming messages
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Print("Read error: ", err)
			return
		}

		var message Message
		if err := json.Unmarshal(msg, &message); err != nil {
			log.Print("Unmarshal error: ", err)
			continue
		}

		switch message.Type {
		case "answer":
			answer := webrtc.SessionDescription{
				Type: webrtc.SDPTypeAnswer,
				SDP:  message.Data,
			}
			fmt.Println("Message Data => => => => ", message.Data)
			if err := peerConnection.SetRemoteDescription(answer); err != nil {
				log.Print("SetRemoteDescription failed: ", err)
				continue
			}
			startMediaPipelines(peerConnection, opusTrack, h264Track)

		case "candidate":
			candidate := webrtc.ICECandidateInit{
				Candidate: message.Data,
			}
			if err := peerConnection.AddICECandidate(candidate); err != nil {
				log.Print("AddICECandidate failed: ", err)
			}
		}
	}
}

func startMediaPipelines(pc *webrtc.PeerConnection, opusTrack, h264Track *webrtc.TrackLocalStaticSample) {
	audioPipeline, err := gst.NewPipelineFromString(
		"pulsesrc ! audioconvert ! audioresample ! audio/x-raw,channels=1,rate=48000 ! opusenc ! appsink name=appsink")
	if err != nil {
		log.Fatal("Audio pipeline failed: ", err)
	}
	audioPipeline.SetState(gst.StatePlaying)
	setupAppSink(audioPipeline, opusTrack)

	videoPipeline, err := gst.NewPipelineFromString(fmt.Sprintf(
		"v4l2src device=%s ! videoconvert ! videoscale ! video/x-raw,width=1280,height=720,framerate=30/1,format=NV12 ! "+
			"nvh264enc preset=low-latency-hq bitrate=%d  ! "+
			"h264parse ! video/x-h264,stream-format=byte-stream ! appsink name=appsink",
		videoDevice, bitrate))
	if err != nil {
		log.Fatal("Video pipeline failed: ", err)
	}
	videoPipeline.SetState(gst.StatePlaying)
	setupAppSink(videoPipeline, h264Track)
}

func setupAppSink(pipeline *gst.Pipeline, track *webrtc.TrackLocalStaticSample) {
	appSink, err := pipeline.GetElementByName("appsink")
	if err != nil {
		log.Fatal("AppSink failed: ", err)
	}

	app.SinkFromElement(appSink).SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
			sample := sink.PullSample()
			if sample == nil {
				return gst.FlowEOS
			}

			buffer := sample.GetBuffer()
			if buffer == nil {
				return gst.FlowError
			}

			samples := buffer.Map(gst.MapRead).Bytes()
			defer buffer.Unmap()

			if err := track.WriteSample(media.Sample{
				Data:     samples,
				Duration: *buffer.Duration().AsDuration(),
			}); err != nil {
				log.Print("WriteSample failed: ", err)
				return gst.FlowError
			}

			return gst.FlowOK
		},
	})
}
