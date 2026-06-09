package audio

import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/moutend/go-wca/pkg/wca"
)

// Config 音频捕获配置
type Config struct {
	SampleRate    int // 默认 16000
	Channels      int // 默认 1 (mono)
	FrameDuration int // 每帧毫秒数，默认 100
}

// DefaultConfig 返回默认音频配置
func DefaultConfig() Config {
	return Config{
		SampleRate:    16000,
		Channels:      1,
		FrameDuration: 100,
	}
}

// Frame 一帧音频数据
type Frame struct {
	Data       []byte
	SampleRate int
	Channels   int
	BitsPerSample int
}

// Capture 系统音频捕获器（WASAPI Loopback）
type Capture struct {
	cfg      Config
	mu       sync.Mutex
	running  bool
	onFrame  func(Frame)
	stopCh   chan struct{}

	// 实际音频格式（由系统混音决定）
	actualSampleRate    int
	actualChannels      int
	actualBitsPerSample int

	// COM 对象
	enumerator    *wca.IMMDeviceEnumerator
	device        *wca.IMMDevice
	audioClient   *wca.IAudioClient
	captureClient *wca.IAudioCaptureClient
}

// NewCapture 创建音频捕获器
func NewCapture(cfg Config) *Capture {
	return &Capture{
		cfg:    cfg,
		stopCh: make(chan struct{}),
	}
}

// SetFrameCallback 设置每帧回调
func (c *Capture) SetFrameCallback(fn func(Frame)) {
	c.mu.Lock()
	c.onFrame = fn
	c.mu.Unlock()
}

// ActualFormat 返回系统实际使用的音频格式
func (c *Capture) ActualFormat() (sampleRate, channels, bitsPerSample int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.actualSampleRate, c.actualChannels, c.actualBitsPerSample
}

// Start 开始捕获系统音频（WASAPI Loopback）
func (c *Capture) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		return nil
	}

	// 1. 初始化 COM
	if err := ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED); err != nil {
		return fmt.Errorf("CoInitializeEx: %w", err)
	}

	// 2. 创建 MMDeviceEnumerator
	var de *wca.IMMDeviceEnumerator
	if err := wca.CoCreateInstance(
		wca.CLSID_MMDeviceEnumerator, 0,
		ole.CLSCTX_ALL,
		wca.IID_IMMDeviceEnumerator,
		&de,
	); err != nil {
		return fmt.Errorf("CoCreateInstance MMDeviceEnumerator: %w", err)
	}
	c.enumerator = de

	// 3. 获取默认渲染设备（用于 Loopback 捕获）
	var mmd *wca.IMMDevice
	if err := de.GetDefaultAudioEndpoint(wca.ERender, wca.EConsole, &mmd); err != nil {
		return fmt.Errorf("GetDefaultAudioEndpoint: %w", err)
	}
	c.device = mmd

	// 4. 激活 IAudioClient
	var ac *wca.IAudioClient
	if err := mmd.Activate(wca.IID_IAudioClient, ole.CLSCTX_ALL, nil, &ac); err != nil {
		return fmt.Errorf("Activate IAudioClient: %w", err)
	}
	c.audioClient = ac

	// 5. 获取系统混音格式（loopback 只能用系统格式）
	var mixFormat *wca.WAVEFORMATEX
	if err := ac.GetMixFormat(&mixFormat); err != nil {
		return fmt.Errorf("GetMixFormat: %w", err)
	}

	// 记录实际音频参数（用于后续 WAV 封装）
	c.actualSampleRate = int(mixFormat.NSamplesPerSec)
	c.actualChannels = int(mixFormat.NChannels)
	c.actualBitsPerSample = int(mixFormat.WBitsPerSample)

	// 6. 初始化音频客户端（Loopback 模式，使用系统原生混音格式）
	hnsBufferDuration := wca.REFERENCE_TIME(c.cfg.FrameDuration) * 10000
	if err := ac.Initialize(
		wca.AUDCLNT_SHAREMODE_SHARED,
		wca.AUDCLNT_STREAMFLAGS_LOOPBACK,
		hnsBufferDuration,
		0,
		mixFormat,
		nil,
	); err != nil {
		return fmt.Errorf("Initialize: %w", err)
	}

	// 7. 获取 IAudioCaptureClient
	var acc *wca.IAudioCaptureClient
	if err := ac.GetService(wca.IID_IAudioCaptureClient, &acc); err != nil {
		return fmt.Errorf("GetService IAudioCaptureClient: %w", err)
	}
	c.captureClient = acc

	// 8. 启动音频客户端
	if err := ac.Start(); err != nil {
		return fmt.Errorf("Start audio client: %w", err)
	}

	c.running = true
	c.stopCh = make(chan struct{})

	// 9. 启动捕获循环
	go c.captureLoop()

	return nil
}

// captureLoop 持续读取音频帧并通过回调发送
func (c *Capture) captureLoop() {
	for {
		select {
		case <-c.stopCh:
			return
		default:
		}

		c.mu.Lock()
		running := c.running
		client := c.captureClient
		fn := c.onFrame
		sampleRate := c.actualSampleRate
		channels := c.actualChannels
		bitsPerSample := c.actualBitsPerSample
		c.mu.Unlock()

		if !running || client == nil {
			return
		}

		var framesInPacket uint32
		if err := client.GetNextPacketSize(&framesInPacket); err != nil {
			continue
		}

		if framesInPacket == 0 {
			continue
		}

		var data *byte
		var framesToRead uint32
		var flags uint32

		if err := client.GetBuffer(&data, &framesToRead, &flags, nil, nil); err != nil {
			continue
		}

		// 跳过静音帧
		if flags&wca.AUDCLNT_BUFFERFLAGS_SILENT == 0 && data != nil && framesToRead > 0 {
			bytesPerFrame := channels * bitsPerSample / 8
			dataLen := int(framesToRead) * bytesPerFrame

			samples := make([]byte, dataLen)
			copy(samples, unsafe.Slice(data, dataLen))

			if fn != nil {
				fn(Frame{
					Data:          samples,
					SampleRate:    sampleRate,
					Channels:      channels,
					BitsPerSample: bitsPerSample,
				})
			}
		}

		client.ReleaseBuffer(framesToRead)
	}
}

// Stop 停止音频捕获
func (c *Capture) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return nil
	}

	c.running = false
	close(c.stopCh)

	if c.audioClient != nil {
		c.audioClient.Stop()
	}
	if c.captureClient != nil {
		c.captureClient.Release()
	}
	if c.audioClient != nil {
		c.audioClient.Release()
	}
	if c.device != nil {
		c.device.Release()
	}
	if c.enumerator != nil {
		c.enumerator.Release()
	}

	ole.CoUninitialize()
	return nil
}

// IsRunning 返回是否正在捕获
func (c *Capture) IsRunning() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running
}
