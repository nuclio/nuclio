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
    "recommonmark",
    "sphinx.ext.autodoc",
    "sphinx.ext.autosummary",
    "sphinx.ext.todo",
    "sphinx.ext.viewcode",
    "python_docs_theme",
    "sphinx_copybutton",
]

templates_path = ["_templates"]
exclude_patterns = ["_build", "Thumbs.db", ".DS_Store"]

language = "go"

# https://sphinx-copybutton.readthedocs.io/en/latest/use.html#strip-and-configure-input-prompts-for-code-cells
copybutton_exclude = ".linenos, .gp, .go"
copybutton_prompt_text = "$ "

source_suffix = {
    ".rst": "restructuredtext",
    ".md": "markdown",
    ".html": "html",
}

master_doc = "contents"

# -- Options for HTML output -------------------------------------------------
# https://www.sphinx-doc.org/en/master/usage/configuration.html#options-for-html-output

html_theme = "sphinx_book_theme"
html_title = ""
html_logo = "assets/images/logo.png"
html_favicon = "./favicon.ico"
extra_navbar = "<p>Your HTML</p>"
nb_execution_mode = "off"
html_sourcelink_suffix = ""
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
    "google_analytics_id": "",
}

html_sidebars = {
    "**": ["navbar-logo.html", "search-field.html", "sbt-sidebar-nav.html"]
}

def setup(app):
    app.connect('source-read', process_tables)


def process_tables(app, docname, source):
    import re
    """
    Convert markdown tables to html, since recommonmark can't. This requires 3 steps:
        Snip out table sections from the markdown
        Convert them to html
        Replace the old markdown table with an html table

    This function is called by sphinx for each document. `source` is a 1-item list. To update the document, replace
    element 0 in `source`.
    """
    import markdown
    md = markdown.Markdown(extensions=['markdown.extensions.tables'])
    table_processor = markdown.extensions.tables.TableProcessor(md.parser, {})

    raw_markdown = source[0]
    blocks = re.split(r'(\n{2,})', raw_markdown)

    for i, block in enumerate(blocks):
        if table_processor.test(None, block):
            html = md.convert(block)
            styled = html.replace('<table>', '<table border="1" class="docutils">', 1)  # apply styling
            blocks[i] = styled

    # re-assemble into markdown-with-tables-replaced
    # must replace element 0 for changes to persist
    source[0] = ''.join(blocks)
