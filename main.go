package main

import (
	"archive/zip"
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/phpdave11/gofpdf"
)

var (
    dejaVuURL       = "blob:https://github.com/da1d194b-43c0-4b05-8668-c7d7bb7f442c"
    dejaVuFontLocal = "DejaVuSans.ttf"
    winArial        = `C:\Windows\Fonts\arial.ttf`
    macArial        = `/Library/Fonts/Arial.ttf`
    macArial2       = `/System/Library/Fonts/Supplemental/Arial.ttf`
    macArial3       = `/System/Library/Fonts/Arial.ttf`
)

var Version = "dev"

func main() {
    // Verificação obrigatória do ffmpeg
    if _, err := exec.LookPath("ffmpeg"); err != nil {
        fmt.Println("==================== FFMPEG NÃO ENCONTRADO ====================")
        fmt.Println("O ffmpeg é obrigatório para conversão dos áudios (.opus para .mp3).")
        fmt.Println("")
        switch runtime.GOOS {
        case "darwin":
            fmt.Println("Para instalar no MacOS, use o Homebrew:")
            fmt.Println("    brew install ffmpeg")
        case "linux":
            fmt.Println("Para instalar no Linux Debian/Ubuntu:")
            fmt.Println("    sudo apt update && sudo apt install ffmpeg")
            fmt.Println("Ou para Fedora/CentOS:")
            fmt.Println("    sudo dnf install ffmpeg")
        case "windows":
            fmt.Println("No Windows, Siga o tutorial:")
            fmt.Println("    https://phoenixnap.com/kb/ffmpeg-windows")
        default:
            fmt.Println("Sistema não reconhecido. Instale ffmpeg conforme seu SO.")
        }
        fmt.Println("===============================================================")
        os.Exit(1)
    }

    if len(os.Args) < 2 || os.Args[1] == "" {
        fmt.Println("USO CORRETO:")
        fmt.Println("  go run main.go /caminho/para/arquivo.zip")
        os.Exit(1)
    }
    zipPath := os.Args[1]
    if stat, err := os.Stat(zipPath); err != nil || stat.IsDir() || !strings.HasSuffix(strings.ToLower(zipPath), ".zip") {
        fmt.Printf("Arquivo informado não é um ZIP válido: %s\n", zipPath)
        os.Exit(1)
    }

    tempDir, err := os.MkdirTemp(".", "whats_zip_temp_")
    if err != nil {
        fmt.Println("Erro criando diretório temporário:", err)
        os.Exit(1)
    }
    defer os.RemoveAll(tempDir)

    if err := unzip(zipPath, tempDir); err != nil {
        fmt.Println("Erro ao descompactar ZIP:", err)
        os.Exit(1)
    }

    // Procura por qualquer arquivo .txt no diretório temporário
    var chatFile string
    err = filepath.Walk(tempDir, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }
        if !info.IsDir() && strings.HasSuffix(strings.ToLower(path), ".txt") {
            chatFile = path
            return filepath.SkipAll
        }
        return nil
    })
    if err != nil {
        fmt.Printf("Erro ao procurar arquivo .txt: %v\n", err)
        os.Exit(1)
    }
    if chatFile == "" {
        fmt.Printf("Nenhum arquivo .txt encontrado no zip extraído (%s)\n", tempDir)
        os.Exit(1)
    }

    outputDir := "output"
    // Limpa a pasta output se ela existir
    if err := os.RemoveAll(outputDir); err != nil {
        fmt.Printf("Erro ao limpar pasta %s: %v\n", outputDir, err)
        os.Exit(1)
    }
    outputMedias := filepath.Join(outputDir, "medias")
    os.MkdirAll(outputMedias, 0755)
    fontPath := assureFont()
    fmt.Println("Usando fonte para PDF:", fontPath)

    messages := parseChat(chatFile)
    mediaMap := processMedias(messages, tempDir, outputMedias)
    pdfPath := filepath.Join(outputDir, "chat_export.pdf")
    generatePDF(messages, mediaMap, outputDir, outputMedias, fontPath)

    absPath, _ := filepath.Abs(pdfPath)
    fmt.Printf("\nPDF gerado com sucesso!\nCaminho completo: %s\n", absPath)
}

func unzip(src, dest string) error {
    r, err := zip.OpenReader(src)
    if err != nil {
        return err
    }
    defer r.Close()
    for _, f := range r.File {
        // Ignora arquivos da pasta __MACOSX
        if strings.Contains(f.Name, "__MACOSX") {
            continue
        }
        fpath := filepath.Join(dest, f.Name)
        if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
            return fmt.Errorf("arquivo %s fora do destino", fpath)
        }
        if f.FileInfo().IsDir() {
            os.MkdirAll(fpath, f.Mode())
            continue
        }
        os.MkdirAll(filepath.Dir(fpath), f.Mode())
        outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
        if err != nil {
            return err
        }
        rc, err := f.Open()
        if err != nil {
            outFile.Close()
            return err
        }
        _, err = io.Copy(outFile, rc)
        outFile.Close()
        rc.Close()
        if err != nil {
            return err
        }
    }
    return nil
}

func assureFont() string {
    if fileExists(dejaVuFontLocal) {
        return dejaVuFontLocal
    }
    fmt.Println("Tentando baixar fonte UTF-8 'DejaVuSans.ttf' (só ocorre uma vez)...")
    resp, err := http.Get(dejaVuURL)
    if err == nil && resp.StatusCode == 200 {
        defer resp.Body.Close()
        fontBytes, err := io.ReadAll(resp.Body)
        if err == nil {
            os.WriteFile(dejaVuFontLocal, fontBytes, 0644)
        }
        if fileExists(dejaVuFontLocal) {
            fmt.Println("Fonte baixada com sucesso!")
            return dejaVuFontLocal
        }
    } else {
        fmt.Println("Não foi possível baixar 'DejaVuSans.ttf'.")
    }
    if runtime.GOOS == "windows" && fileExists(winArial) {
        fmt.Println("Usando Arial do Windows.")
        return winArial
    }
    if runtime.GOOS == "darwin" {
        if fileExists(macArial) {
            fmt.Println("Usando Arial do macOS.")
            return macArial
        }
        if fileExists(macArial2) {
            fmt.Println("Usando Arial (suplem.) do macOS.")
            return macArial2
        }
        if fileExists(macArial3) {
            fmt.Println("Usando Arial (sistema) do macOS.")
            return macArial3
        }
    }
    fmt.Println("\n*** ERRO: Não foi possível obter uma fonte UTF-8 válida. ***")
    fmt.Println("Baixe manualmente 'DejaVuSans.ttf' e coloque na mesma pasta, ou adapte o script para apontar para uma fonte existente.")
    os.Exit(1)
    return ""
}

func fileExists(path string) bool {
    info, err := os.Stat(path)
    if err != nil {
        return false
    }
    return !info.IsDir()
}

type Message struct {
    Time         string
    Sender       string
    Content      string
    Media        string
    MediaIsImage bool
    MediaIsAudio bool
}

func parseChat(chatFile string) []Message {
    file, err := os.Open(chatFile)
    if err != nil {
        panic(err)
    }
    defer file.Close()

    var messages []Message
    scanner := bufio.NewScanner(file)
    
    // Padrão para o primeiro formato: [DD/MM/YYYY, HH:MM:SS] Nome: Mensagem
    msgRegex1 := regexp.MustCompile(`\[(.*?)\] (.*?): (.*)`)
    
    // Padrão para o segundo formato: DD/MM/YYYY HH:MM - Nome: Mensagem
    msgRegex2 := regexp.MustCompile(`(\d{2}/\d{2}/\d{4} \d{2}:\d{2}) - (.*?): (.*)`)
    
    // Padrão para anexos no primeiro formato
    mediaRegex1 := regexp.MustCompile(`<anexado: ([^>]+)>`)
    
    // Padrão para anexos no segundo formato
    mediaRegex2 := regexp.MustCompile(`(.*?) \(arquivo anexado\)`)
    
    // Padrões para diferentes tipos de mídia
    imageRegex := regexp.MustCompile(`(?i)\.(jpg|jpeg|png|gif|bmp|webp)$`)
    audioRegex := regexp.MustCompile(`(?i)\.(opus|mp3|wav|m4a|ogg|aac)$`)

    for scanner.Scan() {
        line := scanner.Text()
        
        // Tenta primeiro o formato 1
        if matches := msgRegex1.FindStringSubmatch(line); matches != nil {
            content := matches[3]
            media := ""
            isImg := false
            isAudio := false
            
            if m := mediaRegex1.FindStringSubmatch(content); m != nil {
                media = m[1]
                content = mediaRegex1.ReplaceAllString(content, "")
                if imageRegex.MatchString(media) {
                    isImg = true
                }
                if audioRegex.MatchString(media) {
                    isAudio = true
                }
            }
            
            messages = append(messages, Message{
                Time:         matches[1],
                Sender:       matches[2],
                Content:      strings.TrimSpace(content),
                Media:        media,
                MediaIsImage: isImg,
                MediaIsAudio: isAudio,
            })
            continue
        }
        
        // Tenta o formato 2
        if matches := msgRegex2.FindStringSubmatch(line); matches != nil {
            content := matches[3]
            media := ""
            isImg := false
            isAudio := false
            
            if m := mediaRegex2.FindStringSubmatch(content); m != nil {
                media = m[1]
                content = mediaRegex2.ReplaceAllString(content, "")
                if imageRegex.MatchString(media) {
                    isImg = true
                }
                if audioRegex.MatchString(media) {
                    isAudio = true
                }
            }
            
            messages = append(messages, Message{
                Time:         matches[1],
                Sender:       matches[2],
                Content:      strings.TrimSpace(content),
                Media:        media,
                MediaIsImage: isImg,
                MediaIsAudio: isAudio,
            })
        }
    }
    return messages
}

func processMedias(messages []Message, inputDir, outputMedias string) map[string]string {
    mediaMap := make(map[string]string)
    
    // Primeiro, vamos criar um mapa de todos os arquivos disponíveis
    availableFiles := make(map[string]string)
    err := filepath.Walk(inputDir, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }
        if !info.IsDir() {
            // Armazena tanto o nome original quanto em lowercase para busca case-insensitive
            baseName := filepath.Base(path)
            availableFiles[baseName] = path
            availableFiles[strings.ToLower(baseName)] = path
            
            // Também armazena versões sem caracteres especiais
            cleanName := strings.Map(func(r rune) rune {
                if r >= 32 && r <= 126 {
                    return r
                }
                return -1
            }, baseName)
            if cleanName != baseName {
                availableFiles[cleanName] = path
                availableFiles[strings.ToLower(cleanName)] = path
            }
        }
        return nil
    })
    if err != nil {
        fmt.Printf("Erro ao listar arquivos: %v\n", err)
        return mediaMap
    }

    for _, msg := range messages {
        if msg.Media == "" {
            continue
        }

        // Tenta encontrar o arquivo de várias formas
        var src string
        mediaName := msg.Media

        // Remove caracteres especiais do nome do arquivo
        cleanMediaName := strings.Map(func(r rune) rune {
            if r >= 32 && r <= 126 {
                return r
            }
            return -1
        }, mediaName)

        // 1. Busca exata
        if path, ok := availableFiles[mediaName]; ok {
            src = path
        } else if path, ok := availableFiles[cleanMediaName]; ok {
            src = path
        } else {
            // 2. Busca case-insensitive
            if path, ok := availableFiles[strings.ToLower(mediaName)]; ok {
                src = path
            } else if path, ok := availableFiles[strings.ToLower(cleanMediaName)]; ok {
                src = path
            } else {
                // 3. Busca por padrão (para arquivos com nomes similares)
                for availableName, path := range availableFiles {
                    if strings.Contains(strings.ToLower(availableName), strings.ToLower(mediaName)) ||
                       strings.Contains(strings.ToLower(availableName), strings.ToLower(cleanMediaName)) {
                        src = path
                        break
                    }
                }
            }
        }

        if src == "" {
            fmt.Printf("Mídia não encontrada: %s\n", msg.Media)
            continue
        }

        // Processa o arquivo baseado na extensão
        ext := strings.ToLower(filepath.Ext(src))
        if ext == ".opus" {
            mp3Name := strings.TrimSuffix(msg.Media, ".opus") + ".mp3"
            dst := filepath.Join(outputMedias, mp3Name)
            if _, err := os.Stat(dst); os.IsNotExist(err) {
                cmd := exec.Command("ffmpeg", "-y", "-i", src, dst)
                fmt.Println("Convertendo", msg.Media, "->", mp3Name)
                if out, err := cmd.CombinedOutput(); err != nil {
                    fmt.Printf("Erro ffmpeg: %s (%s)\n", err, out)
                }
            }
            mediaMap[msg.Media] = mp3Name
        } else {
            dst := filepath.Join(outputMedias, msg.Media)
            if _, err := os.Stat(dst); os.IsNotExist(err) {
                fmt.Println("Copiando", msg.Media, "para output/medias")
                copyFile(src, dst)
            }
            mediaMap[msg.Media] = msg.Media
        }
    }
    return mediaMap
}

func cleanText(text string) string {
    replacements := map[string]string{
        "👍": "[OK]",
        "🔊": "[AUDIO]",
        "📎": "[ARQUIVO]",
        "🏻": "",
        "🏼": "",
        "🏽": "",
        "🏾": "",
        "🏿": "",
        "👋": "[OLA]",
        "❤️": "[CORACAO]",
        "😊": "[SORRISO]",
        "😂": "[RISO]",
        "😍": "[AMOR]",
        "😭": "[CHORO]",
        "😢": "[TRISTE]",
        "😡": "[RAIVA]",
        "😎": "[LEGAL]",
        "🤔": "[PENSANDO]",
        "🙏": "[POR FAVOR]",
        "🎵": "[MUSICA]",
        "📷": "[FOTO]",
        "📹": "[VIDEO]",
        "📱": "[CELULAR]",
        "💪": "[FORCA]",
        "✨": "[BRILHO]",
        "🔥": "[FOGO]",
        "⭐": "[ESTRELA]",
        "✅": "[OK]",
        "❌": "[ERRO]",
        "⚠️": "[ATENCAO]",
        "⚡": "[RAPIDO]",
        "💯": "[100]",
        "🎉": "[FESTA]",
        "🎁": "[PRESENTE]",
        "🎂": "[BOLO]",
        "🎈": "[BALAO]",
        "🎊": "[CONFETES]",
        "🎯": "[ALVO]",
        "🎲": "[DADO]",
        "🎮": "[JOGO]",
        "🎸": "[GUITARRA]",
        "🎹": "[PIANO]",
        "🎺": "[TROMPETE]",
        "🎻": "[VIOLINO]",
        "🎼": "[PARTITURA]",
        "🎧": "[FONE]",
        "🎤": "[MICROFONE]",
        "🎬": "[FILME]",
        "🎭": "[TEATRO]",
        "🎨": "[ARTE]",
        "🎪": "[CIRCO]",
        "🎫": "[INGRESSO]",
        "🎟️": "[TICKET]",
        "🎠": "[CARROSSEL]",
        "🎡": "[RODA GIGANTE]",
        "🎢": "[MONTANHA RUSSA]",
        "🎣": "[PESCA]",
        "🎽": "[CAMISA]",
        "🎾": "[TENIS]",
        "🎿": "[ESQUI]",
        "🏀": "[BASQUETE]",
        "🏈": "[FOOTBALL]",
        "🏉": "[RUGBY]",
        "⚽": "[FUTEBOL]",
        "⚾": "[BASEBALL]",
        "🏐": "[VOLEI]",
        "🏸": "[BADMINTON]",
        "🏓": "[PING PONG]",
        "🏒": "[HOCKEY]",
        "🏑": "[HOCKEY CAMPO]",
        "🏏": "[CRICKET]",
        "🏹": "[ARCO E FLECHA]",
        "⛳": "[GOLFE]",
        "⛸️": "[PATINS]",
        "⛷️": "[ESQUIADOR]",
        "🏂": "[SNOWBOARD]",
        "🏋️": "[MUSCULACAO]",
        "🤼": "[LUTA]",
        "🤸": "[GINASTICA]",
        "⛹️": "[BASQUETE]",
        "🤾": "[HANDEBOL]",
        "🏌️": "[GOLFE]",
        "🏄": "[SURF]",
        "🏊": "[NATACAO]",
        "🤽": "[POLO AQUATICO]",
        "🚣": "[REMO]",
        "🏇": "[HIPISMO]",
        "🚴": "[CICLISMO]",
        "🚵": "[MOUNTAIN BIKE]",
        "🤹": "[MALABARISMO]",
        "🎰": "[CAÇA NÍQUEL]",
        "🎳": "[BOLICHE]",
        "🎱": "[BILHAR]",
    }

    for emoji, replacement := range replacements {
        text = strings.ReplaceAll(text, emoji, replacement)
    }

    var result strings.Builder
    for _, r := range text {
        // Permite apenas caracteres de espaço, pontuação, letras e acentuação comum
        if (r >= 32 && r <= 126) || // ASCII visível
           (r >= 160 && r <= 255) || // Latinos estendidos
           (r == 10) || (r == 13) { // Quebra de linha
            result.WriteRune(r)
        } else {
            // Substitui por espaço para evitar erro
            result.WriteRune(' ')
        }
    }
    return result.String()
}

func generatePDF(messages []Message, mediaMap map[string]string, outputDir, outputMedias, fontPath string) {
    pdf := gofpdf.New("P", "mm", "A4", "")
    pdf.AddPage()
    pdf.AddUTF8Font("custom", "", fontPath)
    pdf.AddUTF8Font("custom", "B", fontPath)
    pdf.SetFont("custom", "B", 16)
    // Título com nome do arquivo ZIP
    zipFile := ""
    if len(os.Args) > 1 {
        zipFile = filepath.Base(os.Args[1])
    }
    if zipFile != "" {
        pdf.SetTextColor(30, 144, 255)
        pdf.CellFormat(0, 12, "Exportação WhatsApp: "+zipFile, "", 1, "C", false, 0, "")
        pdf.Ln(2)
    }
    // Nota sobre links de mídia
    pdf.SetFont("custom", "", 9)
    pdf.SetTextColor(120, 120, 120)
    pdf.MultiCell(0, 5, "Nota: Para abrir mídias em nova aba, clique com o botão direito no link e escolha 'Abrir em nova aba' (comportamento depende do leitor de PDF).", "", "C", false)
    pdf.Ln(2)
    pdf.SetFont("custom", "", 12)

    leftX := 25.0
    rightX := 110.0
    y := pdf.GetY() + 8
    baloonWidth := 80.0
    minBaloonHeight := 18.0
    fontSize := 11.0
    lineHeight := 5.0 // espaçamento mínimo, igual ao tamanho da fonte
    avatarRadius := 7.0
    spaceBetween := 10.0
    lastDate := ""

    for _, msg := range messages {
        // Separador de data
        msgDate := ""
        if len(msg.Time) >= 10 {
            msgDate = msg.Time[:10]
        }
        if msgDate != lastDate && msgDate != "" {
            pdf.SetFillColor(230, 230, 230)
            pdf.SetDrawColor(200, 200, 200)
            pdf.SetTextColor(120, 120, 120)
            pdf.SetFont("custom", "", 9)
            pdf.RoundedRect(60, y, 90, 8, 3, "1234", "F")
            pdf.SetXY(60, y+1)
            pdf.CellFormat(90, 6, msgDate, "", 0, "C", false, 0, "")
            y += 10
            lastDate = msgDate
        }

        senderRight := strings.Contains(strings.ToLower(msg.Sender), "glauco")
        var x float64
        var r, g, b int
        var avatarX float64
        if senderRight {
            x = rightX
            avatarX = x + baloonWidth + 5
            r, g, b = 220, 248, 198 // verde claro
        } else {
            x = leftX
            avatarX = x - avatarRadius*2 - 5
            r, g, b = 245, 245, 245 // cinza claro
        }

        // Avatar com iniciais
        initials := ""
        parts := strings.Fields(msg.Sender)
        for _, p := range parts {
            if len(p) > 0 {
                initials += strings.ToUpper(string(p[0]))
            }
        }
        if len(initials) > 2 {
            initials = initials[:2]
        }

        // --- Calcular altura do balão considerando texto + mídia ---
        totalLines := 0
        for _, para := range strings.Split(cleanText(msg.Content), "\n") {
            lines := pdf.SplitText(para, baloonWidth-12)
            if len(lines) == 0 {
                totalLines++
            } else {
                totalLines += len(lines)
            }
        }
        textHeight := float64(totalLines+1) * lineHeight

        baloonHeight := textHeight + 10
        mediaHeight := 0.0
        imgW := 25.0
        imgH := 25.0
        if msg.Media != "" {
            newName, ok := mediaMap[msg.Media]
            if ok && newName != "" {
                if msg.MediaIsImage && (strings.HasSuffix(strings.ToLower(newName), ".jpg") ||
                    strings.HasSuffix(strings.ToLower(newName), ".jpeg") ||
                    strings.HasSuffix(strings.ToLower(newName), ".png")) {
                    // Ajusta tamanho da imagem para nunca ultrapassar o balão
                    imgW = baloonWidth - 20
                    if imgW > 60 { imgW = 60 } // limite máximo
                    imgH = imgW * 1.0 // quadrada
                    mediaHeight = imgH + 3
                } else {
                    mediaHeight = 12
                }
            } else {
                mediaHeight = 12
            }
        }
        baloonHeight += mediaHeight
        if baloonHeight < minBaloonHeight {
            baloonHeight = minBaloonHeight
        }
        // --- Fim cálculo altura ---

        // Se não couber na página, adiciona nova página antes de desenhar
        if y + baloonHeight + spaceBetween > 270 {
            pdf.AddPage()
            y = 20
        }

        // Avatar
        pdf.SetFillColor(180, 200, 230)
        pdf.SetDrawColor(150, 170, 200)
        pdf.Circle(avatarX+avatarRadius, y+avatarRadius+2, avatarRadius, "FD")
        pdf.SetFont("custom", "B", 9)
        pdf.SetTextColor(10, 10, 10)
        pdf.SetXY(avatarX, y+avatarRadius-4)
        pdf.CellFormat(avatarRadius*2, avatarRadius*2, initials, "", 0, "C", false, 0, "")

        // Sombra do balão
        pdf.SetFillColor(210, 210, 210)
        pdf.RoundedRect(x+2, y+2, baloonWidth, baloonHeight+2, 5, "1234", "F") // sombra

        // Balão de mensagem
        pdf.SetFillColor(r, g, b)
        pdf.SetDrawColor(220, 220, 220)
        pdf.RoundedRect(x, y, baloonWidth, baloonHeight, 5, "1234", "FD")

        // Nome e horário
        pdf.SetXY(x+6, y+2)
        pdf.SetTextColor(10, 10, 10)
        pdf.SetFont("custom", "B", 10)
        pdf.CellFormat(baloonWidth-12, 5, cleanText(msg.Sender), "", 0, "L", false, 0, "")
        pdf.SetFont("custom", "", 8)
        pdf.SetTextColor(120, 120, 120)
        pdf.SetXY(x+baloonWidth-28, y+2)
        pdf.CellFormat(25, 4, cleanText(msg.Time), "", 0, "R", false, 0, "")

        // Conteúdo da mensagem
        pdf.SetXY(x+6, y+8)
        pdf.SetTextColor(60, 60, 60)
        pdf.SetFont("custom", "", fontSize)
        pdf.MultiCell(baloonWidth-12, lineHeight, cleanText(msg.Content), "", "L", false)

        // MIDIAS (agora dentro do balão)
        ymedia := y + textHeight
        if msg.Media != "" {
            newName, ok := mediaMap[msg.Media]
            iconY := ymedia + 2
            iconX := x + 8
            if !ok || newName == "" {
                pdf.SetXY(iconX, iconY)
                pdf.SetTextColor(200, 0, 0)
                pdf.CellFormat(baloonWidth-16, 10, cleanText("[Mídia ausente]"), "", 1, "L", false, 0, "")
            } else {
                mediaRelPath := filepath.Join("medias", newName)
                mediaFullPath := filepath.Join(outputMedias, newName)
                if msg.MediaIsImage && (strings.HasSuffix(strings.ToLower(newName), ".jpg") ||
                    strings.HasSuffix(strings.ToLower(newName), ".jpeg") ||
                    strings.HasSuffix(strings.ToLower(newName), ".png")) {
                    if fileExists(mediaFullPath) {
                        opts := gofpdf.ImageOptions{ImageType: "", ReadDpi: true}
                        // Miniatura da imagem é um link para o arquivo
                        pdf.ImageOptions(mediaFullPath, x+baloonWidth-imgW-5, iconY, imgW, imgH, false, opts, 0, mediaRelPath)
                        pdf.SetXY(iconX, iconY)
                        pdf.SetTextColor(100, 180, 100)
                        pdf.SetFont("custom", "B", 10)
                        pdf.CellFormat(18, 8, cleanText("🖼️"), "", 0, "C", false, 0, mediaRelPath)
                    } else {
                        pdf.SetXY(iconX, iconY)
                        pdf.SetTextColor(200, 0, 0)
                        pdf.CellFormat(baloonWidth-16, 10, cleanText("[imagem ausente]"), "", 1, "L", false, 0, "")
                    }
                } else if msg.MediaIsAudio || strings.HasSuffix(strings.ToLower(newName), ".mp3") {
                    pdf.SetXY(iconX, iconY)
                    if fileExists(mediaFullPath) && newName != "" {
                        pdf.SetTextColor(30, 144, 255)
                        pdf.SetFont("custom", "B", 10)
                        // Ícone de áudio é um link para o arquivo
                        pdf.CellFormat(18, 8, cleanText("🔊"), "", 0, "C", false, 0, mediaRelPath)
                        pdf.SetFont("custom", "", 9)
                        pdf.SetXY(iconX+20, iconY)
                        // Limita o label para não escapar do balão
                        shortName := newName
                        if len(shortName) > 24 {
                            shortName = shortName[:7] + "..." + shortName[len(shortName)-10:]
                        }
                        label := cleanText(fmt.Sprintf("Áudio: %s", shortName))
                        pdf.CellFormat(baloonWidth-38, 8, label, "", 0, "L", false, 0, mediaRelPath)
                    } else {
                        pdf.SetTextColor(200, 0, 0)
                        pdf.CellFormat(baloonWidth-16, 10, cleanText(fmt.Sprintf("[Áudio %s ausente]", newName)), "", 1, "L", false, 0, "")
                    }
                } else {
                    pdf.SetXY(iconX, iconY)
                    if fileExists(mediaFullPath) && newName != "" {
                        pdf.SetTextColor(180, 120, 40)
                        pdf.SetFont("custom", "B", 10)
                        pdf.CellFormat(18, 8, cleanText("📎"), "", 0, "C", false, 0, mediaRelPath)
                        pdf.SetFont("custom", "", 9)
                        pdf.SetXY(iconX+20, iconY)
                        // Limita o label para não escapar do balão
                        shortName := newName
                        if len(shortName) > 24 {
                            shortName = shortName[:7] + "..." + shortName[len(shortName)-10:]
                        }
                        label := cleanText(fmt.Sprintf("Arquivo: %s", shortName))
                        pdf.CellFormat(baloonWidth-38, 8, label, "", 0, "L", false, 0, mediaRelPath)
                    } else {
                        pdf.SetTextColor(200, 0, 0)
                        pdf.CellFormat(baloonWidth-16, 10, cleanText(fmt.Sprintf("[Arquivo %s ausente]", newName)), "", 1, "L", false, 0, "")
                    }
                }
            }
        }
        y = y + baloonHeight + spaceBetween
        pdf.SetTextColor(0, 0, 0)
    }

    pdfPath := filepath.Join(outputDir, "chat_export.pdf")
    if err := pdf.OutputFileAndClose(pdfPath); err != nil {
        fmt.Printf("Erro ao salvar PDF: %v\n", err)
        os.Exit(1)
    }
}

func copyFile(src, dst string) {
    in, err := os.Open(src)
    if err != nil {
        fmt.Printf("Erro abrindo media %s: %v\n", src, err)
        return
    }
    defer in.Close()
    out, err := os.Create(dst)
    if err != nil {
        fmt.Printf("Erro criando media %s: %v\n", dst, err)
        return
    }
    defer out.Close()
    io.Copy(out, in)
}
