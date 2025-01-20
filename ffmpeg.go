package ffmpeg

import (
	"github.com/JoshuaDoes/crunchio"

	"fmt"
	"os/exec"
	"strings"
	"time"
)

var (
	ErrorAlreadyRunning error = fmt.Errorf("ffmpeg: already running")
)

type Ffmpeg struct {
	running   bool
	process   *exec.Cmd
	errors    []error
	onExit    func(ff *Ffmpeg)
	onExitRan bool

	ffmpeg                  string
	input, output           string
	codecIn, codecOut       string
	formatIn, formatOut     string
	channelsIn, channelsOut int
	rateIn, rateOut         int
	bitrateOut              int
	threads                 int
	precision               string
	metadata                map[string]string
	filters                 []*Filter

	audioIn  *crunchio.Buffer
	audioOut *crunchio.Buffer
	stats    *crunchio.Buffer

	buffer  []byte        //Stores the output of the stream until read or flushed
	bufTime time.Duration //length of time to buffer audio for, to keep data in sync with streamers
	bufSize int64         //calculated buffer size of each channel in bytes
}

func NewFFmpeg(codec, format string) *Ffmpeg {
	ff := new(Ffmpeg)
	ff.errors = make([]error, 0)
	ff.buffer = make([]byte, 0)
	ff.filters = make([]*Filter, 0)
	ff.metadata = make(map[string]string)
	ff.codecOut = codec
	ff.formatOut = format
	ff.SetFFmpeg("ffmpeg")
	ff.SetInput("")
	ff.SetOutput("")
	ff.SetBufferStats(crunchio.NewBuffer("stats"))
	return ff
}

func (ff *Ffmpeg) SetFFmpeg(path string) {
	ff.ffmpeg = path
}

func (ff *Ffmpeg) SetBufferAudioIn(buffer *crunchio.Buffer) {
	ff.audioIn = buffer
	if buffer != nil {
		ff.input = ""
	}
}
func (ff *Ffmpeg) GetBufferAudioIn() *crunchio.Buffer {
	if ff.audioIn != nil {
		return ff.audioIn.Reference()
	}
	return nil
}
func (ff *Ffmpeg) SetBufferAudioOut(buffer *crunchio.Buffer) {
	ff.audioOut = buffer
	if buffer != nil {
		ff.output = ""
	}
}
func (ff *Ffmpeg) GetBufferAudioOut() *crunchio.Buffer {
	if ff.audioOut != nil {
		return ff.audioOut.Reference()
	}
	return nil
}
func (ff *Ffmpeg) SetBufferStats(buffer *crunchio.Buffer) {
	ff.stats = buffer
}
func (ff *Ffmpeg) GetBufferStats() *crunchio.Buffer {
	if ff.stats != nil {
		return ff.stats.Reference()
	}
	return nil
}

// Start begins execution of the ffmpeg process and is non-blocking.
func (ff *Ffmpeg) Start() error {
	if ff.IsRunning() {
		return ErrorAlreadyRunning
	}

	process := exec.Command(ff.ffmpeg, ff.Arguments()...)
	if audioIn := ff.GetBufferAudioIn(); audioIn != nil {
		process.Stdin = audioIn
	}
	if audioOut := ff.GetBufferAudioOut(); audioOut != nil {
		process.Stdout = audioOut
	}
	if stats := ff.GetBufferStats(); stats != nil {
		process.Stderr = stats
	}
	ff.process = process

	ff.spawn()
	return nil
}

func (ff *Ffmpeg) spawn() {
	if err := ff.process.Start(); err != nil {
		ff.error(err)
		return
	}
	ff.running = true

	if err := ff.process.Wait(); err != nil {
		ff.error(err)
	}
	ff.running = false

	if ff.onExit != nil {
		ff.onExit(ff)
	}
	ff.onExitRan = true
}

// Close stops the ffmpeg process and cleans up remaining resources.
// Must be called on loop until no error is returned.
func (ff *Ffmpeg) Close() error {
	if ff.process != nil {
		if err := ff.process.Process.Kill(); err != nil {
			return err
		}
		ff.process = nil
	}
	ff.running = false
	for {
		//Wait for onExit callback
		if ff.onExitRan {
			break
		}
	}
	return nil
}

// IsRunning returns true if ffmpeg is currently running.
func (ff *Ffmpeg) IsRunning() bool {
	if ff.process == nil {
		return false
	}
	return ff.running
}

func (ff *Ffmpeg) Run() error {
	if !ff.IsRunning() {
		if err := ff.Start(); err != nil {
			return err
		}
	}
	for {
		if !ff.IsRunning() {
			break
		}
	}
	return nil
}

// SetOnExit sets a callback handler for when ffmpeg exits.
func (ff *Ffmpeg) SetOnExit(fnc func(ff *Ffmpeg)) {
	ff.onExit = fnc
}

func (ff *Ffmpeg) SetBufferLength(d time.Duration) {
	ff.bufTime = d
	sampleRate := int64(ff.rateOut)
	channels := int64(ff.channelsOut)
	var bytesPerSample int64

	switch ff.precision {
	case "f64":
		bytesPerSample = 8
	case "f32":
		bytesPerSample = 4
	default:
		bytesPerSample = 4 // Default to 4 bytes per sample
	}

	ff.bufSize = (int64(d) * sampleRate * channels * bytesPerSample) / int64(time.Second)
}

func (ff *Ffmpeg) SetBufferSize(n int64) {
	ff.bufSize = n
	sampleRate := int64(ff.rateOut)
	channels := int64(ff.channelsOut)
	var bytesPerSample int64

	switch ff.precision {
	case "f64":
		bytesPerSample = 8
	case "f32":
		bytesPerSample = 4
	default:
		bytesPerSample = 4 // Default to 4 bytes per sample
	}

	ff.bufTime = time.Duration((n * int64(time.Second)) / (sampleRate * channels * bytesPerSample))
}

func (ff *Ffmpeg) SetInput(input string) {
	ff.input = input
	if input != "" {
		ff.SetBufferAudioIn(nil)
	} else {
		ff.SetBufferAudioIn(crunchio.NewBuffer("in"))
	}
}
func (ff *Ffmpeg) SetOutput(output string) {
	ff.output = output
	if output != "" {
		ff.SetBufferAudioOut(nil)
	} else {
		ff.SetBufferAudioOut(crunchio.NewBuffer("out"))
	}
}
func (ff *Ffmpeg) SetInputCodec(codec string) {
	ff.codecIn = codec
}
func (ff *Ffmpeg) SetOutputCodec(codec string) {
	ff.codecOut = codec
}
func (ff *Ffmpeg) SetInputFormat(format string) {
	ff.formatIn = format
}
func (ff *Ffmpeg) SetOutputFormat(format string) {
	ff.formatOut = format
}
func (ff *Ffmpeg) SetInputRate(rate int) {
	ff.rateIn = rate
}
func (ff *Ffmpeg) SetOutputRate(rate int) {
	ff.rateOut = rate
}
func (ff *Ffmpeg) SetOutputBitrate(bitrate int) {
	ff.bitrateOut = bitrate
}
func (ff *Ffmpeg) SetThreads(threads int) {
	ff.threads = threads
}
func (ff *Ffmpeg) SetPrecision(precision string) {
	ff.precision = precision
	if len(ff.filters) > 0 {
		for i := 0; i < len(ff.filters); i++ {
			ff.filters[i].SetPrecision(precision)
		}
	}
}
func (ff *Ffmpeg) SetMetadata(key, value string) {
	ff.metadata[key] = value
}

func (ff *Ffmpeg) SetInputChannels(channels int) {
	ff.channelsIn = channels
}
func (ff *Ffmpeg) SetOutputChannels(channels int) {
	ff.channelsOut = channels
}

func (ff *Ffmpeg) Arguments() []string {
	//Prepare list of args
	args := make([]string, 0)
	args = append(args, "-hide_banner", "-stats")

	//Determine real input and output locations
	input := ff.input
	if input == "" {
		input = "pipe:0"
	}
	output := ff.output
	if output == "" {
		output = "pipe:1"
	} else {
		args = append(args, "-y") //Overwrite output without asking
	}

	//Input
	if ff.codecIn != "" {
		args = append(args, "-acodec", ff.codecIn)
	}
	if ff.formatIn != "" {
		args = append(args, "-f", ff.formatIn)
	}
	if ff.channelsIn > 0 {
		args = append(args, "-ac", fmt.Sprintf("%d", ff.channelsIn))
	}
	if ff.rateIn > 0 {
		args = append(args, "-ar", fmt.Sprintf("%d", ff.rateIn))
	}
	args = append(args, "-i", input)

	//Filters
	args = append(args, "-filter_complex")
	args = append(args, ff.generateFilterComplex("a"))
	args = append(args, "-map", "[a]")

	//Metadata
	for key, val := range ff.metadata {
		arg := fmt.Sprintf("%s=%s", key, val)
		args = append(args, "-metadata", arg)
		args = append(args, "-metadata:s:a:0", arg)
	}

	//Output
	args = append(args, "-acodec", ff.codecOut)
	args = append(args, "-f", ff.formatOut)
	if ff.channelsOut > 0 {
		args = append(args, "-ac", fmt.Sprintf("%d", ff.channelsOut))
	}
	if ff.rateOut > 0 {
		args = append(args, "-ar", fmt.Sprintf("%d", ff.rateOut))
	}
	if ff.bitrateOut > 0 {
		args = append(args, "-b:a", fmt.Sprintf("%d", ff.bitrateOut))
	}
	if ff.threads > 0 {
		args = append(args, "-threads", fmt.Sprintf("%d", ff.threads))
	}
	args = append(args, output)

	return args
}

func (ff *Ffmpeg) generateFilterSetting(fs FilterSetting) string {
	str := fs.Name()
	s := fs.Settings()
	if len(s) > 0 {
		str += "="
		for i := 0; i < len(s); i++ {
			v := fs.Get(s[i])
			str += s[i] + "=" + v + ":"
		}
		str = str[:len(str)-1]
	}
	return str
}

func (ff *Ffmpeg) generateFilterComplex(name string) string {
	filters := make([]string, len(ff.filters))

	fc := "[0:a]asplit"
	for i := 0; i < len(ff.filters); i++ {
		filters[i] = fmt.Sprintf("[f%d_0]", i)
		fc += filters[i]
	}
	fc += ";"

	for i := 0; i < len(ff.filters); i++ {
		f := ff.filters[i]
		for j := 0; j < len(f.settings); j++ {
			next := fmt.Sprintf("[f%d_%d]", i, j+1)
			fs := ff.generateFilterSetting(f.settings[j])
			fc += fmt.Sprintf("%s%s%s;", filters[i], fs, next)
			filters[i] = next
		}
	}

	inputs := ""
	pan := make([]string, 0)
	for i := 0; i < len(ff.filters); i++ {
		inputs += filters[i]

		f := ff.filters[i]
		for j := 0; j < len(f.chanOut); j++ {
			pan = append(pan, fmt.Sprintf("c%d=c%d", len(pan), f.chanOut[j]))
		}
	}
	fc += fmt.Sprintf("%samerge=inputs=%d,pan=%dc|%s[%s]", inputs, len(ff.filters), ff.channelsOut, strings.Join(pan, "|"), name)

	return fc
}

func (ff *Ffmpeg) String() string {
	return fmt.Sprintf("ffmpeg\n%s", strings.Join(ff.Arguments(), "\n"))
}

func (ff *Ffmpeg) error(err error) {
	if err != nil {
		ff.errors = append(ff.errors, err)
	}
}

func (ff *Ffmpeg) Error() error {
	errs := ""
	for i := 0; i < len(ff.errors); i++ {
		errs += fmt.Sprintf("%v\n", ff.errors[i])
	}
	if errs == "" {
		return nil
	}
	errs = errs[:len(errs)-1]
	return fmt.Errorf("%s", errs)
}
