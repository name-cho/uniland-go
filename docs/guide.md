# UniLand с нуля

Этот файл — короткий курс. Прочитав его целиком и запустив примеры, вы
сможете писать на UniLand скрипты, вставлять в них Python и Go и передавать
данные между всеми тремя языками.

Все примеры лежат в каталоге `examples` и проверяются тестами: если
документ разойдётся с кодом, `go test ./...` упадёт.

## Запуск

```bash
go build -o uniland .
./uniland run examples/01-basics.uni
```

Есть ещё четыре способа запустить код:

```bash
./uniland -e 'print(2 + 2)'      # одна строка
./uniland -i                     # интерактивный режим
./uniland -c script.uni          # только проверить синтаксис
./uniland run -d script.uni      # напечатать токены и выполнить
```

## Первая программа

```uniland
print("Привет, мир")
let name = "мир"
print(`Привет, {name}!`)
```

`print` принимает сколько угодно аргументов и разделяет их пробелом.
Обратные кавычки делают шаблонную строку: всё, что в `{...}`, вычисляется.

## Переменные и значения

Переменная объявляется через `let`, константа — через `const`. Тип писать
не нужно, он определяется значением.

```uniland
let age = 30
let price = 199.9
let title = "курс"
let active = true
let nothing = null

age = 31
print(age, price, title, active, nothing)
print(typeof(age), typeof(price), typeof(title), typeof(active), typeof(nothing))
```

Вывод:

```
31 199.9 курс true null
number number string boolean null
```

`typeof` возвращает строку: `number`, `string`, `boolean`, `null`, `array`,
`object`, `function`. Числа бывают целыми и дробными, но отдельного типа для
целых нет: `7 / 2` даёт `3.5`, а большие целые вроде `2 ^ 100` считаются
точно, без потери разрядов.

Константу переприсвоить нельзя, это ошибка времени выполнения:

```uniland
const limit = 10
limit = 20
```

У `let` есть область видимости: переменная, объявленная внутри блока `if`
или `while`, снаружи не видна. Переменная из заголовка `for` после цикла
остаётся доступной. Присваивать значение необъявленной переменной нельзя:
получите ошибку `Undefined variable`.

## Строки и шаблоны

Обычные строки — в двойных кавычках. Шаблонные — в обратных, с подстановкой
`{выражение}`. Литеральные фигурные скобки удваиваются: `{{` и `}}`.

```uniland
let user = "Аня"
let items = 3
print(`{user}, у вас {items} товара на сумму {items * 199.9}`)
print("Фигурные скобки: {{так}}")
```

Сложение строк работает через `+`: если хотя бы один операнд — строка,
второй тоже превращается в текст. `"x" + 1` даёт `x1`, а `1 + "x"` — `1x`.
Заранее превратить значение в строку можно через `to_string(1)`.

## Арифметика и сравнения

```uniland
print(10 + 3, 10 - 3, 10 * 3, 10 / 3, 10 % 3, 2 ^ 10)
print(10 > 3, 10 >= 10, 10 == 10, 10 != 3)
print(!true, -5)
```

Обратите внимание на два правила, которые отличаются от многих языков:

- `||` и `&&` имеют одинаковый приоритет, поэтому `a || b && c`
  вычисляется как `(a || b) && c`;
- `^` левоассоциативен: `2 ^ 3 ^ 2` равно `64`, а не `512`.

Оба правила унаследованы от эталонного интерпретатора, и менять их нельзя:
на них построены тесты совместимости.

## Условия

```uniland
let score = 73

if (score >= 90) {
    print("отлично")
} else if (score >= 70) {
    print("хорошо")
} else {
    print("надо повторить")
}
```

Скобки вокруг условия обязательны, фигурные скобки тоже. `else if` можно
писать подряд сколько нужно.

## Циклы

Три формы: `while`, обычный `for` и `for-in` по массиву, строке или объекту.

```uniland
let i = 0
while (i < 3) {
    i++
    print("шаг", i)
}

for (let n = 1; n <= 2; n++) {
    print("for", n)
}

for (item in ["a", "b"]) {
    print("элемент", item)
}

for (key in {name: "Аня", age: 30}) {
    print("поле", key)
}
```

`break` выходит из цикла, `continue` переходит к следующей итерации. Оба
работают и внутри `try`, при этом `finally` всё равно выполнится.

## Массивы

Массив — упорядоченный список. Индексы начинаются с нуля, отрицательные
считаются с конца.

```uniland
let fruits = ["яблоко", "груша", "слива"]
print(fruits[0], fruits[-1], length(fruits))

push(fruits, "вишня")
print(fruits)
print(slice(fruits, 1, 3))
```

Вывод:

```
яблоко слива 3
["яблоко", "груша", "слива", "вишня"]
["груша", "слива"]
```

Массивы передаются по ссылке: если передать массив в функцию и что-то в нём
изменить, изменения увидит вызывающий код. Скалярные значения передаются по
значению.

## Объекты

Объект — набор пар «ключ-значение» с сохранением порядка. Ключ — имя или
строка, доступ через точку или через скобки.

```uniland
let person = {name: "Игорь", city: "Самара"}
person.age = 35
person["job"] = "инженер"
print(person.name, person["city"])
print(person)
```

Вывод:

```
Игорь Самара
{"name": "Игорь", "city": "Самара", "age": 35, "job": "инженер"}
```

Объекты и массивы можно вкладывать друг в друга, а `keys`, `values`,
`entries` и `has` помогают их обходить.

## Функции

Функция объявляется через `func`, параметры — в скобках. `return` вернёт
значение, без него функция вернёт `null`.

```uniland
func greet(name) {
    return `Привет, {name}!`
}

func fact(n) {
    if (n <= 1) { return 1 }
    return n * fact(n - 1)
}

print(greet("Даша"))
print(fact(10))
```

Короткие функции удобно писать лямбдой:

```uniland
let square = x => x * x
let add = (a, b) => a + b
print(square(7), add(2, 3))
```

Лямбда видит переменные, которые были в момент её создания. На этом
строятся счётчики и фабрики функций:

```uniland
func makeCounter() {
    let count = 0
    return () => {
        count += 1
        return count
    }
}

let tick = makeCounter()
tick()
print(tick())
```

Функции — такие же значения, как числа: их можно хранить в массивах и
объектах, передавать аргументами и возвращать.

## Ошибки

Ошибку поднимают через `throw`, ловят через `try/catch`, а `finally`
выполняется в любом случае.

```uniland
func divide(a, b) {
    if (b == 0) {
        throw "на ноль делить нельзя"
    }
    return a / b
}

try {
    print(divide(10, 2))
    print(divide(1, 0))
} catch (e) {
    print("поймали:", e)
} finally {
    print("блок finally выполнился")
}
```

Вывод:

```
5
поймали: на ноль делить нельзя
блок finally выполнился
```

`catch` ловит только ошибки самого UniLand. Встроенные функции тоже
поднимают их: чтение несуществующего файла, разбор плохого JSON или выход
за границу массива попадут в `catch` с понятным текстом.

## Встроенные функции

Их около девяноста, и они покрывают повседневные задачи. Основные группы:

- массивы: `push`, `pop`, `shift`, `unshift`, `slice`, `splice`, `sort`,
  `reverse`, `includes`, `indexOf`, `join`, `sum`, `min`, `max`, `range`,
  `map`, `filter`, `reduce`, `find`, `each`;
- строки: `uppercase`, `lowercase`, `trim`, `split`, `replace`,
  `substring`, `repeat`, `startswith`, `endswith`, `char_code`,
  `from_char_code`;
- объекты: `keys`, `values`, `entries`, `has`, `merge`, `clone`;
- преобразования: `to_string`, `to_number`, `to_boolean`, `typeof`;
- JSON: `parse_json`, `stringify_json`;
- файлы: `read_file`, `write_file`, `append_file`, `file_exists`,
  `list_dir`, `create_dir`, `delete_file`, `copy_file`, `rename_file`;
- сеть и система: `http_get`, `http_post`, `system`, `env`, `argv`, `exit`.

Пример с массивами и строками:

```uniland
let numbers = [4, 1, 3, 2, 5]
print(sort(numbers), sum(numbers))
print(map(numbers, n => n * 2))
print(filter(numbers, n => n % 2 == 0))
print(reduce(numbers, (acc, n) => acc + n, 100))
```

```
[1, 2, 3, 4, 5] 15
[2, 4, 6, 8, 10]
[2, 4]
115
```

Обратите внимание: `sort` и `reverse` меняют сам массив, а не возвращают
копию. Поэтому следующие строки работают уже с отсортированными числами.
Если нужен исходный порядок, сделайте копию через `clone(numbers)`.

## Python внутри UniLand

Блок `python { }` исполняется настоящим интерпретатором Python. Между
блоками живёт одно пространство имён, поэтому импорт, сделанный однажды,
действует дальше.

Обмен идёт через объект `uni`:

- `uni.name` читает глобальную переменную UniLand;
- `uni.name = ...` записывает её;
- `uni.call("func", args...)` вызывает функцию UniLand;
- `uni["name"]` — то же, что `uni.name`, если имя приходит строкой.

```uniland
let counter = 10

python {
    import math
    uni.sqrt2 = round(math.sqrt(2), 4)
    uni.counter = uni.counter + 5
}

print(counter, sqrt2)
```

```
15 1.4142
```

Из UniLand в Python можно попасть и без блока:

```uniland
print(pyeval("2 ** 10"))          # вычислить выражение
pyexec("import math; print('pi:', round(math.pi, 2))")
print(py("1 + 41"))
```

`pyeval` возвращает значение, `pyexec` просто выполняет код и печатает то,
что тот вывел, `py` — короткая форма для выражения.

Обратный вызов, из Python в UniLand:

```uniland
func double(n) {
    return n * 2
}

python {
    uni.doubled = uni.call("double", 21)
}

print(doubled)
```

Функция `double` написана на UniLand, вызвана из Python и вернула `42`.
Аргументы и результат при этом переводятся между типами автоматически.

## Go внутри UniLand

Блок `go { }` компилируется и исполняется через yaegi. Без импортов доступны
`fmt`, `strings`, `strconv`, `math`, `sort`, `time`, `math/big`, `unicode`,
`regexp`.

Обмен — через тот же `uni`, но с явными методами:

- `uni.GetInt`, `uni.GetFloat`, `uni.GetStr`, `uni.GetBool`, `uni.GetBig`;
- `uni.SetInt`, `uni.SetFloat`, `uni.SetStr`, `uni.SetBool`, `uni.SetBig`;
- `uni.Call("func", args...)` вызывает функцию UniLand, аргументы — строки,
  результат тоже строка.

```uniland
let n = 5
let name = "uniland"

go {
    uni.SetInt("n", uni.GetInt("n") * 3)
    uni.SetStr("greeting", "привет, " + strings.ToUpper(uni.GetStr("name")))
}

print(n, greeting)
```

```
15 привет, UNILAND
```

Вызов функции UniLand из Go:

```uniland
func triple(x) {
    return x * 3
}

go {
    result := uni.Call("triple", "7")
    uni.SetStr("triple_s", result)
}

print(triple_s)
```

Отдельно есть `goeval`: он считает выражение на Go и возвращает значение.

```uniland
print(goeval("42 * 2"))
print(goeval(`strings.ToUpper("go")`))
```

```
84
GO
```

Если код Go нужно выполнить без блока, используйте `goexec`. Внутри строки
код Go пишется с обычными двойными кавычками, поэтому саму строку удобно
брать в обратные:

```uniland
goexec(`uni.SetInt("gx", 7)`)
print(gx)
print(goeval("5 * 5"))
```

```
7
25
```

Ошибки внутри Go приходят как обычная ошибка UniLand с префиксом
`Go error:`.

## Все три языка вместе

Смысл всей конструкции в том, что Python и Go могут вызывать одни и те же
функции UniLand, а UniLand видит результаты обоих.

```uniland
let total = 0

func addToTotal(x) {
    total = total + x
    return total
}

python {
    uni.py_value = 10
}

go {
    uni.SetInt("total", uni.GetInt("total") + uni.GetInt("py_value"))
}

go {
    uni.SetStr("go_saw", uni.Call("addToTotal", "5"))
}

python {
    uni.py_saw = uni.call("addToTotal", 7)
}

print(total, go_saw, py_saw)
```

Порядок выполнения: Python кладёт `10` в глобальную переменную, Go
прибавляет её к общей сумме, затем Go и Python по очереди вызывают
`addToTotal` и получают `15` и `22`. Переменная `total` одна на всех.

## Окна

```uniland
let win = gui_window("Анкета", 420, 260)
gui_label(win, "Как вас зовут?")
let name = gui_input(win, "")

gui_button(win, "Отправить", () => {
    alert("Привет, " + gui_get(name))
    gui_close(win)
})

gui_run(win)
```

Окно собирается по частям: `gui_window` создаёт его, `gui_label`,
`gui_input`, `gui_button`, `gui_image` и `gui_gif` добавляют элементы,
`gui_get` читает текст, `gui_set` меняет его, `gui_run` открывает окно и
запускает цикл событий, `gui_close` закрывает. `gui_on_close` вешает
обработчик, `gui_video` открывает файл во внешнем плеере.

Цикл событий запускается один раз: после закрытия окна повторный `gui_run`
возвращает ошибку, поэтому второе окно в одном запуске не открыть.

Системные диалоги: `alert`, `prompt`, `confirm`. Выбор файла или каталога:
`pick_file`, `pick_save`, `pick_folder`.

Окна работают, когда бинарник собран с cgo: на Windows нужен mingw-w64, на
Linux — gcc, `libgl1-mesa-dev` и `xorg-dev`. В сборке без cgo
(`CGO_ENABLED=0`) или с тегом `nogui` функции остаются в реестре, но
возвращают ошибку, которую можно поймать:

```uniland
try {
    gui_window("Окно")
} catch (e) {
    print("окон нет в этой сборке:", e)
}
```

Готовый пример: `uniland run examples/gui/demo.uni`.

## Форматтер и сервер для редактора

```bash
uniland fmt script.uni        # показать отформатированный текст
uniland fmt -w script.uni     # перезаписать файл
uniland fmt -l script.uni     # проверить, код 1 если есть что править
uniland lsp                   # запустить Language Server (stdio)
```

Форматтер меняет только отступы, хвостовые пробелы и пустые строки. Код при
этом не переставляется, комментарии и тела `python { }` / `go { }`
сохраняются. Сервер показывает ошибки синтаксиса и умеет форматировать
документ по запросу редактора.

## Шпаргалка

```uniland
let x = 1                 const y = 2
x = 3                     x += 1
print(a, b)               typeof(a)
`текст {выражение}`       "строка"
[1, 2, 3]                 {key: "value"}
if (cond) { } else { }    while (cond) { }
for (let i = 0; i < 3; i++) { }
for (item in list) { }
func f(a, b) { return a + b }
x => x * 2                (a, b) => a + b
try { } catch (e) { } finally { }
throw "текст"
python { uni.x = 1 }      pyeval("1 + 1")
go { uni.SetInt("x", 1) } goeval("1 + 1")
```

## Куда смотреть дальше

- `docs/COMPAT.md` — чем поведение отличается от эталонного интерпретатора.
- `docs/grammar/uniland.ebnf` — формальная грамматика.
- `examples/` — девять запускаемых программ, из которых собран этот текст.
