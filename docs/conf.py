# Configuration file for the Sphinx documentation builder.
#
# For the full list of built-in configuration values, see the documentation:
# https://www.sphinx-doc.org/en/master/usage/configuration.html

# -- Project information -----------------------------------------------------
# https://www.sphinx-doc.org/en/master/usage/configuration.html#project-information

project = 'nuclio'
copyright = '2023, nuclio'
author = 'nuclio'
release = '1.12.3'

# -- General configuration ---------------------------------------------------
# https://www.sphinx-doc.org/en/master/usage/configuration.html#general-configuration

extensions = [

    "sphinx.ext.napoleon",
    "recommonmark",
    "sphinx.ext.autodoc",
    "sphinx.ext.autosummary",
    "sphinx.ext.todo",
    "sphinx.ext.viewcode",
    'python_docs_theme',
]

templates_path = ['_templates']
exclude_patterns = ['_build', 'Thumbs.db', '.DS_Store']

language = 'go'

source_suffix = {
    ".rst": "restructuredtext",
    '.md': 'markdown',
}


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
    "extra_navbar": 'By <a href="https://www.iguazio.com/">Iguazio</a>',
    "extra_footer": "",
    "google_analytics_id": "",
}

html_sidebars = {
    "**": ["navbar-logo.html", "search-field.html", "sbt-sidebar-nav.html"]
}