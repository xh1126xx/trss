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

// DefaultConfig 返回默认音频配置（16kHz mono, 100ms per frame）
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
}

// Capture 系统音频捕获器（WASAPI Loopback）
type Capture struct {
	cfg      Config
	mu       sync.Mutex
	running  bool
	onFrame  func(Frame)
	stopCh   chan struct{}

	// COM 对象
	enumerator    *wca.IMMDeviceEnumerator
	device        *wca.IMMDevice
	audioClient   *wca.IAudioClient
	captureClient *wca.IAudioCaptureClient
	mixFormat     *wca.WAVEFORMATEX
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

	// 5. 获取混音格式作为参考
	var mixFormat *wca.WAVEFORMATEX
	if err := ac.GetMixFormat(&mixFormat); err != nil {
		return fmt.Errorf("GetMixFormat: %w", err)
	}
	c.mixFormat = mixFormat

	// 6. 初始化音频客户端（Loopback 模式）
	// hnsBufferDuration = FrameDuration * 10000 (hundreds of nanoseconds)
	hnsBufferDuration := wca.REFERENCE_TIME(c.cfg.FrameDuration) * 10000
	if err := ac.Initialize(
		wca.AUDCLNT_SHAREMODE_SHARED,
		wca.AUDCLNT_STREAMFLAGS_LOOPBACK,
		hnsBufferDuration,
		0, // 共享模式时 nsPeriodicity 必须为 0
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
		c.mu.Unlock()

		if !running || client == nil {
			return
		}

		// 检查是否有可用的音频帧
		var framesInPacket uint32
		if err := client.GetNextPacketSize(&framesInPacket); err != nil {
			continue
		}

		if framesInPacket == 0 {
			// 没有数据，短暂等待避免忙轮询
			// Sleep approximately for the frame duration
			continue
		}

		// 读取音频数据
		var data *byte
		var framesToRead uint32
		var flags uint32
		var devicePos, qpcPos uint64

		if err := client.GetBuffer(&data, &framesToRead, &flags, nil, nil); err != nil {
			continue
		}

		// 跳过静音帧
		if flags&wca.AUDCLNT_BUFFERFLAGS_SILENT == 0 && data != nil && framesToRead > 0 {
			c.mu.Lock()
			rate := c.cfg.SampleRate
			c.mu.Unlock()

			// 计算数据大小：帧数 * 通道数 * 每样本字节数
			c.mu.Lock()
			channels := c.cfg.Channels
			c.mu.Unlock()

			bytesPerFrame := channels * 2 // 16-bit = 2 bytes per sample per channel
			dataLen := int(framesToRead) * bytesPerFrame

			// 从 C 指针安全复制数据到 Go 切片
			samples := make([]byte, dataLen)
			copy(samples, unsafe.Slice(data, dataLen))

			if fn != nil {
				fn(Frame{
					Data:       samples,
					SampleRate: rate,
				})
			}
		}

		client.ReleaseBuffer(framesToRead)
		_ = devicePos
		_ = qpcPos
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

	// 停止并释放 COM 资源
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
