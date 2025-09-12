# Configuration file for the Sphinx documentation builder.
#
# For the full list of built-in configuration values, see the documentation:
# https://www.sphinx-doc.org/en/master/usage/configuration.html

# -- Project information -----------------------------------------------------
# https://www.sphinx-doc.org/en/master/usage/configuration.html#project-information

project = "nuclio"
copyright = "2023, Iguazio"
author = "nuclio"
release = "1.12.8"

# -- General configuration ---------------------------------------------------
# https://www.sphinx-doc.org/en/master/usage/configuration.html#general-configuration

extensions = [
    "sphinx.ext.napoleon",
    "myst_parser",
    "sphinx.ext.autodoc",
    "sphinx.ext.autosummary",
    "sphinx.ext.todo",
    "sphinx.ext.viewcode",
    "python_docs_theme",
    "sphinx_copybutton",
]


# Enable extended syntax for links, tables, footnotes, etc.
myst_enable_extensions = [
    "colon_fence",        # ```{python} style code blocks
    "deflist",            # definition lists
    "linkify",            # auto-convert URLs to links
    "substitution",       # substitution syntax
    "tasklist",           # GitHub-style task lists
    "html_admonition",   # allows HTML-style admonitions
    "html_image",        # supports HTML <img> tags
]

# Automatically convert .md links to .html in HTML build
myst_html_meta = {
    "enable_auto_links": "true"
}
templates_path = ["_templates"]
exclude_patterns = ["_build", "Thumbs.db", ".DS_Store"]
myst_xref_missing = "ignore"

nitpick_ignore = [
    ("any", "spec.build.codeEntryType"),  # add more as needed
]

suppress_warnings = ["myst.header"]

linkcheck_ignore = [
    r'https:\/\/github\.com\/.*\/.*#L\d+-L\d+',
    # linkcheck doesn't work well with relative paths which contain anchor, so ignore them
    r'^.*\.html#.*$',
    r'^\./[^/]+\.html#.*$',
    r'^\.\./[^/]+\.html#.*$',
    # ignore links to kubernetes.io, since they often block the traffic
    r"https://kubernetes.io/.*",

    "https://github.com/grafana/azure-monitor-datasource/blob/master/README.md#configure-the-data-source",
    "https://github.com/GoogleContainerTools/kaniko/blob/main/README.md#additional-flags",

]
linkcheck_anchors = True
linkcheck_timeout = 60

language = "go"

# https://sphinx-copybutton.readthedocs.io/en/latest/use.html#strip-and-configure-input-prompts-for-code-cells
copybutton_exclude = ".linenos, .gp, .go"
copybutton_prompt_text = "$ "

source_suffix = {
    ".rst": "restructuredtext",
    ".md": "markdown",
}

# Add any paths that contain custom static files (such as style sheets) here,
# relative to this directory. They are copied after the builtin static files,
# so a file named "default.css" will overwrite the builtin "default.css".
html_static_path = ["_static", "assets"]
html_css_files = ["custom.css"]

master_doc = "contents"

# -- Options for HTML output -------------------------------------------------
# https://www.sphinx-doc.org/en/master/usage/configuration.html#options-for-html-output

html_theme = "sphinx_book_theme"
html_title = ""
html_logo = "assets/images/logo.png"
html_favicon = "./favicon.ico"
nb_execution_mode = "off"
autoclass_content = "both"

html_theme_options = {
    "github_url": "https://github.com/nuclio/nuclio",
    "repository_url": "https://github.com/nuclio/nuclio",
    "use_repository_button": True,
    "use_issues_button": True,
    "use_edit_page_button": True,
    "path_to_docs": "docs",
    "home_page_in_toc": False,
    "repository_branch": "development",
    "show_navbar_depth": 1,
    "extra_footer": "",
}
myst_heading_anchors = 5

html_sidebars = {
    "**": ["navbar-logo.html", "search-field.html", "sbt-sidebar-nav.html"]
}


def setup(app):
    pass
