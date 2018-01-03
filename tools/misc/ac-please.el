; Auto-complete mode for BUILD files.
; Heavily based on ac-python.el.

(defun ac-please-get-symbol-at-point ()
  "Return symbol at point. Assumes symbol can be alphanumeric, `.' or `_'."
  (let ((end (point))
        (start (ac-please-start-of-expression)))
    (buffer-substring-no-properties start end)))

(defun ac-please-completion-at-point ()
  "Returns a possibly empty list of completions for the symbol at point."
                                        ;(build-symbol-completions (ac-please-get-symbol-at-point)))
  (message "%s" (ac-please-get-symbol-at-point))
  (message "%s" (ac-please-start-of-expression))
  (message "%s" (ac-please-start-of-function))
  )

(defun ac-please-start-of-expression ()
  "Return point of the start of build expression at point. Assumes symbol can be alphanumeric, `.' or `_'."
  (save-excursion
    (and (re-search-backward
          (rx (or buffer-start (regexp "[^[:alnum:]._]"))
              (group (1+ (regexp "[[:alnum:]._]"))) point)
          nil t)
         (match-beginning 1))))

(defun ac-please-start-of-function ()
  "Return point of the start of build expression at point. Assumes symbol can be alphanumeric, `.' or `_'."
  (save-excursion
    (re-search-backward "^[[:alnum:]_]+(" (match-beginning 1))))

(defvar ac-source-please
  '((candidates . ac-please-completion-at-point)
    (prefix . ac-please-start-of-expression)
    (symbol . "f")
    (requires . 2))
  "Source for build completion.")

(add-to-list 'ac-modes 'please-mode)
(add-hook 'please-mode-hook (lambda () (add-to-list 'ac-sources 'ac-source-please)))

(provide 'ac-please)
