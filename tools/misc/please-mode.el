; Definition of Please mode for Emacs.
;
; Add this to .emacs to make this load automatically.
;(add-to-list 'auto-mode-alist '("/BUILD\\'" . please-mode))
; And add this to run buildifier on the current buffer when saving:
; (add-hook 'after-save-hook 'please-buildify-on-save).

(require 'auto-complete)
(require 'python)

(defvar please-mode-hook nil)

(defvar please-mode-map
  (let ((map (make-keymap)))
    map)
  "Keymap for Please major mode")

(defconst please-font-lock-keywords-1
  (list
   '("#\\(.*\\)$" . font-lock-comment-face)
   '("\\<\\(a\\(?:nd\\|ssert\\)\\|continue\\|def\\|el\\(?:if\\|se\\)\\|for\\|i[fns]\\|not\\|or\\|pass\\|r\\(?:aise\\|eturn\\)\\)\\>" . font-lock-keyword-face)
   '("\\<\\(False\\|None\\|True\\)\\>" . font-lock-constant-face)
   '("\\<\\(bool\\|int\\|list\\|dict\\|str\\)\\>" . font-lock-type-face)
   '("\\<\\(a\\(?:dd_\\(?:dep\\|exported_dep\\|licence\\|out\\)\\|ll\\|ny\\)\\|b\\(?:\\(?:asenam\\|uild_rul\\)e\\)\\|c\\(?:_\\(?:binary\\|embed_binary\\|library\\|object\\|s\\(?:hared_object\\|tatic_library\\)\\|test\\)\\|anonicalise\\|c_\\(?:binary\\|embed_binary\\|library\\|object\\|s\\(?:hared_object\\|tatic_library\\)\\|test\\)\\|go_\\(?:library\\|test\\)\\|heck_config\\|o\\(?:nfig_\\(?:get\\|setting\\)\\|py\\|unt\\)\\)\\|d\\(?:e\\(?:bug\\|compose\\)\\|irname\\)\\|e\\(?:n\\(?:dswith\\|umerate\\)\\|rror\\|xport_file\\)\\|f\\(?:a\\(?:\\(?:i\\|ta\\)l\\)\\|i\\(?:legroup\\|nd\\)\\|ormat\\)\\|g\\(?:e\\(?:n\\(?:rule\\|test\\)\\|t\\(?:_\\(?:base_path\\|command\\|labels\\)\\)?\\)\\|lob\\|o_\\(?:binary\\|get\\|library\\|test\\)\\|rpc_l\\(?:anguages\\|ibrary\\)\\)\\|h\\(?:as\\(?:_label\\|h_filegroup\\)\\|ttp_archive\\)\\|i\\(?:nfo\\|sinstance\\|tems\\)\\|j\\(?:ava_\\(?:binary\\|library\\|module\\|runtime_image\\|test\\)\\|oin\\(?:_path\\)?\\)\\|keys\\|l\\(?:en\\|o\\(?:ad\\|wer\\)\\|strip\\)\\|maven_jars?\\|n\\(?:\\(?:ew_http_archiv\\|otic\\)e\\)\\|p\\(?:a\\(?:ckage\\(?:_name\\)?\\|rtition\\)\\|ip_library\\|roto\\(?:_l\\(?:anguages?\\|ibrary\\)\\|c_binary\\)\\|ython_\\(?:binary\\|library\\|test\\|wheel\\)\\)\\|r\\(?:ange\\|e\\(?:\\(?:mote_fil\\|plac\\)e\\)\\|find\\|partition\\|strip\\)\\|s\\(?:e\\(?:lect\\|t\\(?:_command\\|default\\)\\)\\|h_\\(?:binary\\|cmd\\|library\\|test\\)\\|orted\\|plit\\(?:_path\\|ext\\)?\\|t\\(?:artswith\\|rip\\)\\|ub\\(?:include\\|repo\\)\\|ystem_library\\)\\|tarball\\|upper\\|values\\|w\\(?:arning\\|orkspace\\)\\|zip\\)\\>" . font-lock-builtin-face)
   '("\\([a-zA-Z0-9_]+\\)" . font-lock-variable-name-face)
   '("\\(\\(?:\"\\|'\\|\"\"\"\\|'''\\).*(?:\"\\|'\\|\"\"\"\\|''')\\)" . font-lock-string-face)
   '("def *\\([a-zA-Z0-9_]+\\) *(" 1 font-lock-function-name-face)
  "Highlighting expressions for Please mode"))

(defvar please-font-lock-keywords please-font-lock-keywords-1
  "Highlighting expressions for Please mode")

(defun please-indent-line ()
  "Indent current line as a Please BUILD file"
  (interactive)
  (beginning-of-line)
  (if (bobp)
      (indent-lint-to 0)  ; First line is non-indented
	(let ((not-indented t) cur-indent)
	  (if (looking-at "^ *)") ; If the line we are looking at is the end of a block, then decrease the indentation
		  (progn
			(save-excursion
			  (forward-line -1)
			  (setq cur-indent (- (current-indentation) 4)))
			(if (< cur-indent 0) ; We can't indent past the left margin
				(setq cur-indent 0)))
		(save-excursion
		  (while not-indented ; Iterate backwards until we find an indentation hint
			(forward-line -1)
			(if (looking-at "^ *)") ; This hint indicates that we need to indent at the level of the END_ token
				(progn
				  (setq cur-indent (current-indentation))
				  (setq not-indented nil))
			  (if (looking-at "^.*:") ; This hint indicates that we need to indent an extra level
				  (progn
					(setq cur-indent (+ (current-indentation) 4)) ; Do the actual indenting
					(setq not-indented nil))
				(if (bobp)
					(setq not-indented nil)))))))
	  (if cur-indent
		  (indent-line-to cur-indent)
		(indent-line-to 0))))) ; If we didn't see an indentation hint, then allow no indentation

(defvar please-mode-syntax-table
  (let ((table (make-syntax_table)))
            table)
  "Syntax table for please-mode")
(modify-syntax-entry ?_ "w" please-mode-syntax-table)
(modify-syntax-entry ?/ "w" please-mode-syntax-table)
(modify-syntax-entry ?: "w" please-mode-syntax-table)

(defun please-mode ()
  (interactive)
  (kill-all-local-variables)
  (use-local-map please-mode-map)
  (set-syntax-table please-mode-syntax-table)
  (set (make-local-variable 'font-lock-defaults) '(please-font-lock-keywords))
  ;(set (make-local-variable 'indent-line-function) 'please-indent-line)
  ; using Python's indent for now
  (set (make-local-variable 'indent-line-function) 'python-indent-line)
  (setq major-mode 'please-mode)
  (setq mode-name "Please")
  (run-hooks 'please-mode-hook))


;;  Autocompletion support.

(defun ac-please-is-label (label)
  (or (string-prefix-p "//" label) (string-prefix-p ":" label)))

(defun ac-please-labels (prefix)
  (message "ac-please-labels %s" prefix)
  (if (ac-please-is-label prefix)
      (process-lines "plz" "query" "completions" prefix)
    (list)))

(defconst ac-please-keywords
  (sort
   (list "build_rule" "len" "enumerate" "zip" "join" "split" "replace" "partition" "rpartition"
   "startswith" "endswith" "format" "lstrip" "rstrip" "strip" "find" "rfind" "count" "upper"
   "lower" "fail" "subinclude" "load" "subrepo" "isinstance" "range" "any" "all" "glob" "package"
   "sorted" "get" "setdefault" "config_get" "get_base_path" "package_name" "canonicalise" "keys"
   "values" "items" "copy" "debug" "info" "notice" "warning" "error" "fatal" "join_path"
   "split_path" "splitext" "basename" "dirname" "get_labels" "has_label" "add_dep"
   "add_exported_dep" "add_out" "add_licence" "get_command" "set_command" "cc_library" "cc_object"
   "cc_static_library" "cc_shared_object" "cc_binary" "cc_test" "cc_embed_binary" "select"
   "config_setting" "c_library" "c_object" "c_static_library" "c_shared_object" "c_binary" "c_test"
   "c_embed_binary" "go_library" "cgo_library" "go_binary" "go_test" "cgo_test" "go_get"
   "java_library" "java_module" "java_runtime_image" "java_binary" "java_test" "maven_jars"
   "maven_jar" "genrule" "gentest" "export_file" "filegroup" "hash_filegroup" "system_library"
   "remote_file" "tarball" "decompose" "check_config" "proto_library" "grpc_library"
   "proto_language" "proto_languages" "grpc_languages" "protoc_binary" "python_library"
   "python_binary" "python_test" "pip_library" "python_wheel" "sh_library" "sh_binary" "sh_test"
   "sh_cmd" "workspace" "http_archive" "new_http_archive")
   #'(lambda (a b) (> (length a) (length b))))
  "builtin functions and keywords.")

(defun ac-please-candidates ()
  (append nil ac-please-keywords (ac-please-labels ac-target)))

(defun ac-please-prefix ()
  (or (ac-prefix-symbol) (ac-please-is-label (thing-at-point 'word))))

(ac-define-source please
  '((candidates . ac-please-candidates)
    (candidate-face . ac-candidate-face)
    (selection-face . ac-selection-face)
    ;(document . ac-please-document)
    ;(action . ac-please-action)
    (prefix . ac-please-prefix)
    (requires . 0)
    (cache)))


(add-hook 'please-mode-hook (lambda () (add-to-list 'ac-sources 'ac-source-please)))
(add-to-list 'ac-modes 'please-mode)

;; Autoformatting
(defun please-buildify ()
  "Format the current buffer according to the buildifier tool,
  in a pretty quick and dirty way."
  (interactive)
  (call-process "buildifier" nil nil nil "-mode=fix" (buffer-file-name))
  (revert-buffer t t))

;;;###autoload
(defun please-buildify-on-save ()
  (interactive)
  (when (eq major-mode 'please-mode) (please-buildify)))

(provide 'please-mode)
