; Python syntax highlighting queries
; Based on tree-sitter-python highlight queries

; Keywords
[
  "and" "as" "assert" "async" "await" "break" "class" "continue"
  "def" "del" "elif" "else" "except" "finally" "for" "from"
  "global" "if" "import" "in" "is" "lambda" "nonlocal" "not"
  "or" "pass" "raise" "return" "try" "while" "with" "yield"
] @keyword

; Built-in constants
[
  (none)
  (true)
  (false)
] @constant.builtin

; Built-in exceptions
((identifier) @type.builtin
 (#match? @type.builtin "^(ArithmeticError|AssertionError|AttributeError|BaseException|BlockingIOError|BrokenPipeError|BufferError|BytesWarning|ChildProcessError|ConnectionAbortedError|ConnectionError|ConnectionRefusedError|ConnectionResetError|DeprecationWarning|EOFError|Ellipsis|EnvironmentError|Exception|FileExistsError|FileNotFoundError|FloatingPointError|FutureWarning|GeneratorExit|IOError|ImportError|ImportWarning|IndentationError|IndexError|InterruptedError|IsADirectoryError|KeyError|KeyboardInterrupt|LookupError|MemoryError|ModuleNotFoundError|NameError|NotADirectoryError|NotImplemented|NotImplementedError|OSError|OverflowError|PendingDeprecationWarning|PermissionError|ProcessLookupError|RecursionError|ReferenceError|ResourceWarning|RuntimeError|RuntimeWarning|StopAsyncIteration|StopIteration|SyntaxError|SyntaxWarning|SystemError|SystemExit|TabError|TimeoutError|TypeError|UnboundLocalError|UnicodeDecodeError|UnicodeEncodeError|UnicodeError|UnicodeTranslateError|UnicodeWarning|UserWarning|ValueError|Warning|WindowsError|ZeroDivisionError)$"))

; Function definitions
(function_definition
  name: (identifier) @function)

; Function calls
(call
  function: (identifier) @function)

(call
  function: (attribute
    attribute: (identifier) @function.method))

; Built-in functions
((call
  function: (identifier) @function.builtin)
 (#match? @function.builtin "^(abs|all|any|ascii|bin|bool|breakpoint|bytearray|bytes|callable|chr|classmethod|compile|complex|delattr|dict|dir|divmod|enumerate|eval|exec|filter|float|format|frozenset|getattr|globals|hasattr|hash|help|hex|id|input|int|isinstance|issubclass|iter|len|list|locals|map|max|memoryview|min|next|object|oct|open|ord|pow|print|property|range|repr|reversed|round|set|setattr|slice|sorted|staticmethod|str|sum|super|tuple|type|vars|zip|__import__)$"))

; Decorators
(decorator) @function

; Class definitions
(class_definition
  name: (identifier) @type)

; Constants (ALL_CAPS identifiers)
((identifier) @constant
 (#match? @constant "^[A-Z][A-Z_]*$"))

; Type annotations
(type (identifier) @type)

; Variables
(identifier) @variable

; Literals
(integer) @number
(float) @number
(string) @string
(concatenated_string) @string

; Comments
(comment) @comment

; Operators
[
  "+"
  "-"
  "*"
  "/"
  "//"
  "%"
  "**"
  "="
  "+="
  "-="
  "*="
  "/="
  "//="
  "%="
  "**="
  "=="
  "!="
  "<"
  "<="
  ">"
  ">="
  "<<"
  ">>"
  "&"
  "|"
  "^"
  "~"
  "<<="
  ">>="
  "&="
  "|="
  "^="
] @operator

; Punctuation
[
  "("
  ")"
  "["
  "]"
  "{"
  "}"
  ","
  ":"
  ";"
  "."
] @punctuation.delimiter