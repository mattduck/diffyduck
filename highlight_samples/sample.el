;;; sample.el --- Sample Emacs Lisp for syntax highlighting -*- lexical-binding: t; -*-

;; Copyright (C) 2024  Example Author

;; Author: Example Author <author@example.com>
;; Version: 1.0.0
;; Keywords: sample, highlighting

;;; Commentary:

;; This is a sample Emacs Lisp file demonstrating syntax highlighting
;; for various Elisp constructs.

;;; Code:

(require 'cl-lib)

;; Variables
(defvar sample-greeting "Hello"
  "The greeting to use.")

(defconst sample-version "1.0.0"
  "The version of this sample.")

(defcustom sample-name "World"
  "The name to greet."
  :type 'string
  :group 'sample)

;; Functions
(defun sample-greet (name)
  "Greet NAME with a message."
  (interactive "sName: ")
  (message "%s, %s!" sample-greeting name))

(defun sample-factorial (n)
  "Calculate the factorial of N."
  (if (<= n 1)
      1
    (* n (sample-factorial (1- n)))))

;; Lambda and higher-order functions
(defun sample-map-double (numbers)
  "Double all NUMBERS in the list."
  (mapcar (lambda (x) (* x 2)) numbers))

;; Macros
(defmacro sample-when-let (binding &rest body)
  "Bind BINDING and execute BODY if non-nil."
  (declare (indent 1))
  `(let ((,(car binding) ,(cadr binding)))
     (when ,(car binding)
       ,@body)))

;; Conditions and control flow
(defun sample-classify-number (n)
  "Classify N as positive, negative, or zero."
  (cond
   ((> n 0) 'positive)
   ((< n 0) 'negative)
   (t 'zero)))

;; Structures
(cl-defstruct sample-person
  name
  age
  email)

;; Let bindings
(defun sample-process-data ()
  "Process some sample data."
  (let* ((numbers '(1 2 3 4 5))
         (doubled (sample-map-double numbers))
         (sum (apply #'+ doubled)))
    (message "Sum of doubled numbers: %d" sum)))

;; Hooks
(add-hook 'emacs-startup-hook
          (lambda ()
            (message "Sample module loaded!")))

(provide 'sample)
;;; sample.el ends here
