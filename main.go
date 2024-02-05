package main

import (
	"crypto/aes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/bogem/id3v2"
	"github.com/flopp/go-findfont"
	"github.com/go-flac/flacpicture"
	"github.com/go-flac/flacvorbis"
	"github.com/go-flac/go-flac"
	"image/color"
	"log"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"
)

var (
	corekey        string        = "687A4852416D736F356B496E62617857"
	metaKey        string        = "2331346C6A6B5F215C5D2630553C2728"
	logEntry       *widget.Entry // 引用到 logEntry 组件
	selectedFolder string        // 保存选择的文件夹路径
	folderLabel    *widget.Label // 用于显示选择的文件夹路径
)

func init() {
	// 查找并设置中文字体路径
	fontPaths := findfont.List()
	for _, path := range fontPaths {
		if strings.Contains(path, "simkai.ttf") {
			err := os.Setenv("FYNE_FONT", path)
			if err != nil {
				log.Println("设置字体全局变量异常")
			}
			break
		}
	}
	err := os.Mkdir("_Music", 0750)
	if err != nil && !os.IsExist(err) {
		log.Fatal(err)
	}
}

func main() {
	myApp := app.New()

	myWindow := myApp.NewWindow("网易云音乐转换")
	// 取消窗口最大化
	myWindow.SetFixedSize(true)

	//设置app 图口默认图标
	myWindow.SetIcon(theme.FyneLogo())

	// 创建一个文本区域用于显示日志
	logEntry = widget.NewMultiLineEntry()
	logEntry.Disable() // 禁用编辑功能
	logEntry.Resize(fyne.NewSize(413, 320))
	logEntry.Move(fyne.NewPos(5, 60))

	// Label 用于显示选择的文件夹路径
	folderLabel = widget.NewLabel("PATH: ")
	folderLabel.Resize(fyne.NewSize(200, 40))
	folderLabel.Move(fyne.NewPos(5, 7))

	// Button 用于选择文件夹
	folderButton := widget.NewButton("打开", func() {
		showFolderDialog(myWindow)
	})
	folderButton.Resize(fyne.NewSize(60, 40))
	folderButton.Move(fyne.NewPos(283, 5))

	// Button 用于执行操作
	startButton := widget.NewButton("开始", func() {
		if selectedFolder != "" {
			Run(selectedFolder)
		} else {
			//log.Println("请先选择文件夹路径")
			setLogText("请先选择文件夹路径")
		}
	})
	startButton.Resize(fyne.NewSize(60, 40))
	startButton.Move(fyne.NewPos(353, 5))

	// Label 用于显示版权信息
	authorLabel := canvas.NewText("本软件仅供学习和个人使用，禁止用于商业用途。作者: Minshenyao", color.Black)
	authorLabel.TextSize = 13
	authorLabel.Move(fyne.NewPos(14, 385))

	myWindow.SetContent(
		container.NewWithoutLayout(
			folderLabel,
			startButton,
			folderButton,
			logEntry,
			authorLabel,
		),
	)
	//myApp.Settings().SetTheme(theme.DarkTheme())
	myWindow.Resize(fyne.NewSize(430, 410)) // 设置窗口大小

	// 关闭窗口时的回调函数
	myWindow.SetCloseIntercept(func() {
		// 取消字体全局变量
		if err := os.Unsetenv("FYNE_FONT"); err != nil {
			log.Println("取消字体全局变量异常")
		}

		// 退出应用程序
		myApp.Quit()
	})
	myWindow.ShowAndRun()
}

func setLogText(text string) {
	now := time.Now().Format("2006/01/02 15:04:05 ")
	logEntry.Append(now + text + "\n")
}

// showFolderDialog 显示文件夹选择对话框
func showFolderDialog(window fyne.Window) {
	dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
		if err == nil && uri != nil {
			// 获取文件夹路径并更新 Label
			selectedFolder = uri.Path()
			//log.Println("选择的文件夹路径:", selectedFolder)
			folderLabel.SetText("PATH: " + selectedFolder)
			setLogText("选择的文件夹路径:" + selectedFolder)
		}
	}, window)
}

func Run(inputPath string) {
	outputPath := inputPath + "/_Music"
	err := filepath.Walk(inputPath,
		func(path string, info os.FileInfo, err error) error {
			defer func() {
				if r := recover(); r != nil {
					fmt.Println("转换失败：", path)
					setLogText("转换失败: " + path)
				}
			}()

			if err != nil {
				return err
			}

			extname := filepath.Ext(path)
			if extname == ".ncm" {
				fmt.Println(path, outputPath)
				result := DecodeNCM(path, outputPath)
				setLogText(result)
			}
			//else if extname == ".qmcflac" || extname == ".qmc0" {
			//	decoder.DecodeQMC(path, outputPath)
			//}
			return nil
		})
	if err != nil {
		return
	}
}

func DecodeNCM(filePath string, outputFolder string) string {
	f, err := os.Open(filePath)
	if err != nil {
		fmt.Println(err)
	}
	defer f.Close()

	header := make([]byte, 8)
	_, err = f.Read(header)
	if err != nil {
		fmt.Println(err)
	}

	if string(header) != string(Unhexlify("4354454e4644414d")) {
		fmt.Println("不是正常的ncm格式！")
	}

	_, err = f.Seek(2, 1)
	if err != nil {
		fmt.Println(err)
	}

	keyLengthBytes := make([]byte, 4)
	f.Read(keyLengthBytes)
	keyLength := binary.LittleEndian.Uint16(keyLengthBytes)
	keyData := make([]byte, keyLength)
	f.Read(keyData)
	newKeyData := make([]byte, keyLength)
	for i, b := range keyData {
		newKeyData[i] = b ^ 0x64
	}

	decryptedKeyData := aesDecryptECB(Unhexlify(corekey), newKeyData)
	newDecryptKey := decryptedKeyData[17:]
	newKeyLength := len(newDecryptKey)

	s := make([]byte, 256)
	for i := 0; i < 256; i++ {
		s[i] = byte(i)
	}
	j := 0

	for i := 0; i < 256; i++ {
		j = (j + int(s[i]) + int(newDecryptKey[i%newKeyLength])) & 0xFF
		s[i], s[j] = s[j], s[i]
	}

	// identifier := ""
	metaLengthBytes := make([]byte, 4)
	f.Read(metaLengthBytes)
	metaLength := binary.LittleEndian.Uint16(metaLengthBytes)
	metaDataMap := make(map[string]interface{})

	info, err := os.Stat(filePath)
	if err != nil {
		fmt.Println(err)
	}

	format := "mp3"

	if metaLength > 0 {
		metaData := make([]byte, metaLength)
		f.Read(metaData)
		newMetaData := make([]byte, metaLength)
		for i, b := range metaData {
			newMetaData[i] = b ^ 0x63
		}

		// identifier = string(newMetaData)
		realMeataData, err := base64.StdEncoding.DecodeString(string(newMetaData[22:]))
		if err != nil {
			fmt.Println(err)
		}
		decodeMetaData := aesDecryptECB(Unhexlify(metaKey), realMeataData)
		json.Unmarshal(decodeMetaData[6:], &metaDataMap)
	} else {
		if int64(info.Size()) > int64(1024*1024*16) {
			format = "flac"
		}
		metaDataMap = map[string]interface{}{
			"format": format,
		}
	}

	if metaFormat, ok := metaDataMap["format"]; ok {
		format = metaFormat.(string)
	}

	f.Seek(5, 1)
	imageSpaceBytes := make([]byte, 4)
	f.Read(imageSpaceBytes)
	imageSpace := binary.LittleEndian.Uint32(imageSpaceBytes)
	imageSizeBytes := make([]byte, 4)
	f.Read(imageSizeBytes)
	imageSize := binary.LittleEndian.Uint32(imageSizeBytes)

	imageDataBytes := make([]byte, imageSize)
	if imageSize > 0 {
		f.Read(imageDataBytes)
	}

	pos, err := f.Seek(int64(imageSpace-imageSize), 1)
	if err != nil {
		fmt.Println(err)
	}

	dataLen := info.Size() - pos
	data := make([]byte, dataLen)
	f.Read(data)

	stream := make([]byte, 256)
	for i := 0; i < 256; i++ {
		j := (int(s[i]) + int(s[(i+int(s[i]))&0xFF])) & 0xFF
		stream[i] = s[j]
	}

	newStream := make([]byte, 0)
	for i := 0; i < len(data); i++ {
		v := stream[(i+1)%256]
		newStream = append(newStream, v)
	}

	newData := strxor(string(data), string(newStream))

	extname := filepath.Ext(filePath)
	basename := filepath.Base(filePath)
	filename := strings.TrimSuffix(basename, extname)

	newFilename := filename + "." + format

	outPath := path.Join(outputFolder, newFilename)
	of, _ := os.Create(outPath)
	defer of.Close()
	_, err = of.Write([]byte(newData))
	if err != nil {
		fmt.Println(err)
	}
	of.Sync()

	artistName := ""
	if artists, ok := metaDataMap["artist"]; ok {
		tp := reflect.TypeOf(artists)
		nameArr := make([]string, 0)
		switch tp.Kind() {
		case reflect.Slice, reflect.Array:
			items := reflect.ValueOf(artists)
			for i := 0; i < items.Len(); i++ {
				item := items.Index(i).Interface()
				tp1 := reflect.TypeOf(item)
				switch tp1.Kind() {
				case reflect.Slice, reflect.Array:
					values := reflect.ValueOf(item)
					if values.Len() > 0 {
						nameArr = append(nameArr, values.Index(0).Interface().(string))
					}
				}
			}
		}
		artistName = strings.Join(nameArr, "/")
	}

	imageFormat := "image/jpeg"
	if string(imageDataBytes[0:4]) == string(Unhexlify("89504E47")) {
		imageFormat = "image/png"
	}

	if format == "mp3" {
		mp3File, err := id3v2.Open(outPath, id3v2.Options{Parse: false})
		defer mp3File.Close()

		if err != nil {
			fmt.Println(err)
		}

		mp3File.SetDefaultEncoding(id3v2.EncodingUTF8)
		mp3File.SetArtist(artistName)
		mp3File.SetTitle(fmt.Sprintf("%v", metaDataMap["musicName"]))
		mp3File.SetAlbum(fmt.Sprintf("%v", metaDataMap["album"]))

		if len(imageDataBytes) > 0 {
			pic := id3v2.PictureFrame{
				Encoding:    id3v2.EncodingISO,
				MimeType:    imageFormat,
				PictureType: id3v2.PTFrontCover,
				Description: "Front cover",
				Picture:     imageDataBytes,
			}
			mp3File.AddAttachedPicture(pic)
		}

		if err = mp3File.Save(); err != nil {
			fmt.Println(err)
		}

	} else {
		flacFile, err := flac.ParseFile(outPath)
		if err != nil {
			fmt.Println(err)
		}

		cmts, idx := extractFLACComment(outPath)
		if cmts == nil && idx > 0 {
			cmts = flacvorbis.New()
		}
		cmts.Add(flacvorbis.FIELD_TITLE, fmt.Sprintf("%v", metaDataMap["musicName"]))
		cmts.Add(flacvorbis.FIELD_ALBUM, fmt.Sprintf("%v", metaDataMap["album"]))
		cmts.Add(flacvorbis.FIELD_ARTIST, artistName)

		cmtsmeta := cmts.Marshal()
		if idx > 0 {
			flacFile.Meta[idx] = &cmtsmeta
		} else {
			flacFile.Meta = append(flacFile.Meta, &cmtsmeta)
		}

		var pic *flacpicture.MetadataBlockPicture
		pic = extractFLACCover(outPath)
		if pic != nil {
			pic.ImageData = imageDataBytes
		} else {
			pic, err = flacpicture.NewFromImageData(flacpicture.PictureTypeFrontCover, "Front cover", imageDataBytes, imageFormat)
			if err != nil {
				fmt.Println(err)
			}
			picturemeta := pic.Marshal()
			flacFile.Meta = append(flacFile.Meta, &picturemeta)
		}

		err = flacFile.Save(outPath)
		if err != nil {
			fmt.Println(err)
		}

	}

	fmt.Println(basename, "->", newFilename)

	return basename + "->" + newFilename
}

func extractFLACComment(fileName string) (*flacvorbis.MetaDataBlockVorbisComment, int) {
	f, err := flac.ParseFile(fileName)
	if err != nil {
		fmt.Println(err)
	}

	var cmt *flacvorbis.MetaDataBlockVorbisComment
	var cmtIdx int
	for idx, meta := range f.Meta {
		if meta.Type == flac.VorbisComment {
			cmt, err = flacvorbis.ParseFromMetaDataBlock(*meta)
			cmtIdx = idx
			if err != nil {
				fmt.Println(err)
			}
		}
	}
	return cmt, cmtIdx
}

func extractFLACCover(fileName string) *flacpicture.MetadataBlockPicture {
	f, err := flac.ParseFile(fileName)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	var pic *flacpicture.MetadataBlockPicture
	for _, meta := range f.Meta {
		if meta.Type == flac.Picture {
			pic, err = flacpicture.ParseFromMetaDataBlock(*meta)
			if err != nil {
				fmt.Println(err)
			}
		}
	}
	return pic
}

// func NewImageFrame(ft idv23.FrameType, mime_type string, image_data []byte) *idv23.ImageFrame {
//     data_frame := idv23.NewDataFrame(ft, image_data)
//     data_frame.size += uint32(1)
//
//     // ID3 standard says the string has to be null-terminated.
//     nullTermBytes := append(image_data, 0x00)
//
//     image_frame := &idv23.ImageFrame{
//         DataFrame:   *data_frame, // DataFrame header
//         pictureType: byte(0x03),  // Image Type, in this case Front Cover (http://id3.org/id3v2.3.0#Attached_picture)
//         description: string(nullTermBytes),
//     }
//     image_frame.SetEncoding("UTF-8")
//     image_frame.SetMIMEType(mime_type)
//     return image_frame
// }
//

func strxor(s1, s2 string) string {
	if len(s1) != len(s2) {
		panic("strXor called with two strings of different length\n")
	}
	n := len(s1)
	b := make([]byte, n)
	for i := 0; i < n; i++ {
		b[i] = s1[i] ^ s2[i]
	}
	return string(b)
}

func Unpad(data []byte, blockSize uint) ([]byte, error) {
	if blockSize < 1 {
		return nil, fmt.Errorf("Block size looks wrong")
	}

	if uint(len(data))%blockSize != 0 {
		return nil, fmt.Errorf("Data isn't aligned to blockSize")
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("Data is empty")
	}

	paddingLength := int(data[len(data)-1])
	for _, el := range data[len(data)-paddingLength:] {
		if el != byte(paddingLength) {
			return nil, fmt.Errorf("Padding had malformed entries. Have '%x', expected '%x'", paddingLength, el)
		}
	}

	return data[:len(data)-paddingLength], nil
}

func generateKey(key []byte) (genKey []byte) {
	genKey = make([]byte, 16)
	copy(genKey, key)
	for i := 16; i < len(key); {
		for j := 0; j < 16 && i < len(key); j, i = j+1, i+1 {
			genKey[j] ^= key[i]
		}
	}
	return genKey
}

func aesDecryptECB(key []byte, encrypted []byte) []byte {
	cipher, _ := aes.NewCipher(generateKey(key))
	decrypted := make([]byte, len(encrypted))

	for bs, be := 0, cipher.BlockSize(); bs < len(encrypted); bs, be = bs+cipher.BlockSize(), be+cipher.BlockSize() {
		cipher.Decrypt(decrypted[bs:be], encrypted[bs:be])
	}

	trim := 0
	if len(decrypted) > 0 {
		trim = len(decrypted) - int(decrypted[len(decrypted)-1])
	}

	return decrypted[:trim]
}

func Unhexlify(str string) []byte {
	res := make([]byte, 0)
	for i := 0; i < len(str); i += 2 {
		x, _ := strconv.ParseInt(str[i:i+2], 16, 32)
		res = append(res, byte(x))
	}
	return res
}
