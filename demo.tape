# VHS documentation
#
# Output:
#   Output <path>.gif               Create a GIF output at the given <path>
#   Output <path>.mp4               Create an MP4 output at the given <path>
#   Output <path>.webm              Create a WebM output at the given <path>
#
# Require:
#   Require <string>                Ensure a program is on the $PATH to proceed
#
# Settings:
#   Set FontSize <number>           Set the font size of the terminal
#   Set FontFamily <string>         Set the font family of the terminal
#   Set Height <number>             Set the height of the terminal
#   Set Width <number>              Set the width of the terminal
#   Set LetterSpacing <float>       Set the font letter spacing (tracking)
#   Set LineHeight <float>          Set the font line height
#   Set LoopOffset <float>%         Set the starting frame offset for the GIF loop
#   Set Theme <json|string>         Set the theme of the terminal
#   Set Padding <number>            Set the padding of the terminal
#   Set Framerate <number>          Set the framerate of the recording
#   Set PlaybackSpeed <float>       Set the playback speed of the recording
#   Set MarginFill <file|#000000>   Set the file or color the margin will be filled with.
#   Set Margin <number>             Set the size of the margin. Has no effect if MarginFill isn't set.
#   Set BorderRadius <number>       Set terminal border radius, in pixels.
#   Set WindowBar <string>          Set window bar type. (one of: Rings, RingsRight, Colorful, ColorfulRight)
#   Set WindowBarSize <number>      Set window bar size, in pixels. Default is 40.
#   Set TypingSpeed <time>          Set the typing speed of the terminal. Default is 50ms.
#
# Sleep:
#   Sleep <time>                    Sleep for a set amount of <time> in seconds
#
# Type:
#   Type[@<time>] "<characters>"    Type <characters> into the terminal with a
#                                   <time> delay between each character
#
# Keys:
#   Escape[@<time>] [number]        Press the Escape key
#   Backspace[@<time>] [number]     Press the Backspace key
#   Delete[@<time>] [number]        Press the Delete key
#   Insert[@<time>] [number]        Press the Insert key
#   Down[@<time>] [number]          Press the Down key
#   Enter[@<time>] [number]         Press the Enter key
#   Space[@<time>] [number]         Press the Space key
#   Tab[@<time>] [number]           Press the Tab key
#   Left[@<time>] [number]          Press the Left Arrow key
#   Right[@<time>] [number]         Press the Right Arrow key
#   Up[@<time>] [number]            Press the Up Arrow key
#   Down[@<time>] [number]          Press the Down Arrow key
#   PageUp[@<time>] [number]        Press the Page Up key
#   PageDown[@<time>] [number]      Press the Page Down key
#   Ctrl+<key>                      Press the Control key + <key> (e.g. Ctrl+C)
#
# Display:
#   Hide                            Hide the subsequent commands from the output
#   Show                            Show the subsequent commands in the output

Output demo.gif

Require echo
Require dfd

Set Shell "bash"
Set Theme "GruvboxDark"
Set FontSize 14
#Set FontFamily "Monaco"         # Set the font family of the terminal
Set Width 1420
Set Height 925
Set Framerate 30
Set Padding 25
Set PlaybackSpeed 0.4
#Set BorderRadius 10

Hide
Type "dfd show cbbb792" Sleep 50ms  Enter
Enter
Sleep 500ms

Type "jjjj"
Tab Sleep 100ms
Tab Sleep 100ms
Tab Sleep 100ms

Type "gj"
Type "gj"

Show

Sleep 200ms
Type "j" Sleep 200ms
Type "j" Sleep 100ms
Type "kk" Sleep 50ms
Tab Sleep 200ms

Type "jjjj" Sleep 50ms
Type "jjjj" Sleep 50ms

Type "lll" Sleep 200ms
Type "hhh" Sleep 150ms

Type "jjjj" Sleep 100ms

Ctrl+d Sleep 350ms
Ctrl+d Sleep 350ms
Ctrl+d Sleep 350ms

Space
Type "cj" Sleep 500ms

Enter
Sleep 500ms
Space
Space

Type "edit"

Sleep 500ms
Backspace
Backspace
Backspace
Backspace
Backspace
Backspace
Sleep 500ms
Ctrl+c

Sleep 500ms

Type "/cursor"
Sleep 300ms
Enter Sleep 500ms
Type "n" Sleep 200ms
Type "n" Sleep 500ms

Ctrl+h Sleep 2000ms
Type "q" Sleep 200ms

Type "Y"
Sleep 500ms
