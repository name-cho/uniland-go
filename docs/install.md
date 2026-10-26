# Установка

Собрать UniLand можно на Windows, Linux и в Termux. Нужен только Go 1.23
или новее; Python нужен, только если скрипты используют блоки
`python { }`, а C-компилятор — только для окон на Fyne. Без компилятора
собирайте с `CGO_ENABLED=0` или `-tags nogui`.

Готовые сборки не публикуются, поэтому все команды ниже собирают бинарник
из исходников.

## Общее

```bash
git clone https://github.com/name-cho/uniland-go.git
cd uniland-go
go build -trimpath -ldflags "-s -w" -o uniland .
./uniland -v
```

Если `uniland -v` печатает `UniLand 2.2.0`, сборка удалась.

Есть и вариант без клонирования, через `go install`:

```bash
go install github.com/name-cho/uniland-go@v0.1.0
```

Так бинарник называется по имени модуля — `uniland-go`, и лежит в
`$(go env GOPATH)/bin`. При желании переименуйте его в `uniland`.

## Windows

1. Установите Go: `winget install GoLang.Go` либо установщик с go.dev.
   Проверка в новом терминале: `go version`.
2. Соберите бинарник в каталоге репозитория:

   ```powershell
   go build -trimpath -ldflags "-s -w" -o uniland.exe .
   .\uniland.exe -v
   ```

3. Положите его в личный каталог и добавьте этот каталог в `PATH`:

   ```powershell
   $dir = "$env:USERPROFILE\bin"
   New-Item -ItemType Directory -Force $dir | Out-Null
   Copy-Item .\uniland.exe $dir -Force
   $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
   if ($userPath -notlike "*$dir*") {
       [Environment]::SetEnvironmentVariable("Path", "$userPath;$dir", "User")
   }
   ```

4. Закройте и откройте терминал заново, затем проверьте:

   ```powershell
   uniland -v
   uniland -e "print(2 + 2)"
   ```

Если Go ставился установщиком, каталог `%USERPROFILE%\go\bin` обычно уже
в `PATH` — тогда шаг 3 не нужен, достаточно `go install`.

Python для блоков `python { }` ставится с python.org; подойдёт и версия из
Microsoft Store, но её заглушки интерпретатор отфильтровывает, поэтому
лучше обычный установщик. Проверка: `python --version`.

Окна требуют mingw-w64: `choco install mingw` или установщик с
winlibs.com. После установки компилятора обычный `go build -o uniland.exe .`
соберёт бинарник с окнами. Без компилятора добавьте `CGO_ENABLED=0` (или
`-tags nogui`) — получится заглушка, а `gui_*` будут возвращать ошибку.

## Linux

Пакеты для сборки:

```bash
sudo apt install golang git      # Debian, Ubuntu
sudo dnf install golang git      # Fedora
sudo pacman -S go git            # Arch, Manjaro
```

Сборка и установка в систему:

```bash
git clone https://github.com/name-cho/uniland-go.git
cd uniland-go
go build -trimpath -ldflags "-s -w" -o uniland .
sudo install -Dm755 uniland /usr/local/bin/uniland
uniland -v
```

`/usr/local/bin` уже есть в `PATH` и не занят пакетами дистрибутива, поэтому
это удобное место для собранного вручную бинарника. Обновление — повторить
`go build` и `install`, удаление — `sudo rm /usr/local/bin/uniland`.

Если root не нужен, ставьте в домашний каталог:

```bash
mkdir -p ~/.local/bin
install -m755 uniland ~/.local/bin/uniland
```

Каталог `~/.local/bin` должен быть в `PATH`; в большинстве дистрибутивов он
там уже есть, иначе добавьте в `~/.profile`:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

Python для блоков `python { }`: `sudo apt install python3`, `sudo pacman -S python`
или аналогичная команда вашего дистрибутива. Свой интерпретатор можно
указать явно:

```bash
UNILAND_PYTHON=/usr/bin/python3.13 uniland run script.uni
```

Окна включаются автоматически, если есть cgo и системные библиотеки. На
Debian и Ubuntu нужны `gcc`, `libgl1-mesa-dev` и `xorg-dev`, на Arch —
`base-devel` и `libx11`:

```bash
sudo apt install gcc libgl1-mesa-dev xorg-dev
go build -o uniland .
```

Без этих пакетов собирайте с `CGO_ENABLED=0` или `-tags nogui`: тогда
`gui_*` возвращают ошибку.

### Arch и пакетный менеджер

Файл в `/usr/local/bin` не принадлежит ни одному пакету, поэтому `pacman -Qo
/usr/local/bin/uniland` скажет, что владелец неизвестен. Это нормально для
ручной сборки.

Если хочется, чтобы бинарник ставился и удалялся через pacman, сделайте
локальный PKGBUILD:

```bash
pkgname=uniland-go
pkgver=0.1.0
pkgrel=1
pkgdesc="Интерпретатор UniLand на Go"
arch=('x86_64' 'aarch64')
url="https://github.com/name-cho/uniland-go"
license=('MIT')
depends=('glibc')
makedepends=('go')
source=("$pkgname-$pkgver.tar.gz::$url/archive/refs/tags/v$pkgver.tar.gz")
sha256sums=('SKIP')

build() {
    cd "$srcdir/uniland-go-$pkgver"
    export CGO_ENABLED=0
    go build -trimpath -ldflags "-s -w" -o uniland .
}

package() {
    cd "$srcdir/uniland-go-$pkgver"
    install -Dm755 uniland "$pkgdir/usr/bin/uniland"
}
```

Затем `makepkg -si`. В пакете бинарник ставится в `/usr/bin`, а не в
`/usr/local/bin`, как и положено пакетам.

## Termux

Termux — это Android, поэтому окон не будет, а всё остальное работает.
Сборка идёт без cgo:

```bash
pkg update
pkg install golang git
git clone https://github.com/name-cho/uniland-go.git
cd uniland-go
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o "$PREFIX/bin/uniland" .
uniland -v
```

`$PREFIX/bin` уже в `PATH`, поэтому бинарник сразу доступен по имени.
Обновление — `git pull` и повторная сборка, удаление —
`rm "$PREFIX/bin/uniland"`.

Python:

```bash
pkg install python
uniland -e 'python { uni.x = 1 }
print(x)'
```

Что важно знать про Termux:

- окон `gui_*` нет: бинарник собран без cgo, поэтому функции возвращают
  ошибку «GUI недоступен в этой сборке», и это ожидаемо (Fyne в Termux не
  собирается);
- доступ к общей памяти телефона открывается командой `termux-setup-storage`,
  после неё скрипты видят `~/storage/shared`;
- сборка идёт заметно дольше, чем на компьютере, а кэш Go занимает место в
  домашнем каталоге Termux.

## Проверка после установки

```bash
uniland -v
uniland -e 'print(2 + 2)'
uniland run examples/01-basics.uni
uniland fmt -l examples/01-basics.uni
```

Первая команда печатает версию, вторая — `4`, третья прогоняет пример из
курса, четвёртая проверяет, отформатирован ли файл (код 0 — да).
