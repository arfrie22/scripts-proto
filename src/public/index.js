// index.js
const APP = {
    state: {
        isAuthenticated: false,
        currentView: null,
        validations: {
            projectName: false,
            subdomain: false,
            containerImage: false
        }
    },

    utils: {
        debounce(func, wait) {
            let timeout;
            return function executedFunction(...args) {
                const later = () => {
                    clearTimeout(timeout);
                    func(...args);
                };
                clearTimeout(timeout);
                timeout = setTimeout(later, wait);
            };
        },

        showView(templateId) {
            const app = document.getElementById('app');
            const template = document.getElementById(templateId);
            app.innerHTML = template.innerHTML;
            this.initializeCurrentView();
        }
    },

    auth: {
        isAuthenticated() {
            return !!localStorage.getItem('token');
        },

        login(username, password) {
            return fetch('/api/login', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ username, password })
            })
            .then(response => response.json())
            .then(data => {
                if (data.token) {
                    localStorage.setItem('token', data.token);
                    APP.state.isAuthenticated = true;
                    const returnUrl = sessionStorage.getItem('returnUrl') || '/';
                    sessionStorage.removeItem('returnUrl');
                    APP.router.navigate(returnUrl);
                }
            });
        },

        logout() {
            localStorage.removeItem('token');
            APP.state.isAuthenticated = false;
            window.location.href = '/login';
        }
    },

    router: {
        init() {
            window.addEventListener('hashchange', this.handleRoute.bind(this));
            window.addEventListener('popstate', this.handleRoute.bind(this));
            this.handleRoute();
        },

        navigate(path) {
            if (path === '/login') {
                window.location.href = '/login';
            } else {
                window.location.hash = path;
            }
        },

        handleRoute() {
            if (window.location.pathname === '/login') {
                if (APP.auth.isAuthenticated()) {
                    const returnUrl = sessionStorage.getItem('returnUrl') || '/';
                    sessionStorage.removeItem('returnUrl');
                    this.navigate(returnUrl);
                    return;
                }
                APP.views.login.init();
                return;
            }

            if (!APP.auth.isAuthenticated()) {
                sessionStorage.setItem('returnUrl', window.location.hash.slice(1) || '/');
                window.location.href = '/login';
                return;
            }

            const path = window.location.hash.slice(1) || '/';

            switch (path) {
                case '/':
                case '/apps':
                    APP.views.appsList.init();
                    break;
                case '/create':
                    APP.views.createApp.init();
                    break;
                default:
                    this.navigate('/');
            }
        }
    },

    views: {
        login: {
            init() {
                APP.utils.showView('login-view');
                this.attachEventListeners();
            },

            attachEventListeners() {
                const form = document.getElementById('loginForm');
                form.addEventListener('submit', (e) => {
                    e.preventDefault();
                    const username = document.getElementById('username').value;
                    const password = document.getElementById('password').value;
                    APP.auth.login(username, password);
                });
            }
        },

        appsList: {
            init() {
                APP.utils.showView('apps-list-view');
                this.loadApps();
                this.attachEventListeners();
            },

            loadApps() {
                fetch('/api/listApps', {
                    headers: {
                        'Authorization': `Bearer ${localStorage.getItem('token')}`
                    }
                })
                .then(response => response.json())
                .then(apps => {
                    const appsList = document.getElementById('appsList');
                    appsList.innerHTML = apps.map(app => `
                        <div class="app-item">
                            <h3>${app.projectName}</h3>
                            <p>URL: ${app.subdomain}.scripts.mkr.cx</p>
                            <p>Image: ${app.containerImage}</p>
                        </div>
                    `).join('');
                });
            },

            attachEventListeners() {
                document.getElementById('newAppBtn').addEventListener('click', () => {
                    APP.router.navigate('/create');
                });
            }
        },

        createApp: {
            init() {
                APP.utils.showView('create-app-view');
                this.attachEventListeners();
            },

            async validateField(field, endpoint) {
                const value = document.getElementById(field).value;
                const errorElement = document.getElementById(`${field}Error`);
                const successElement = document.getElementById(`${field}Success`);

                try {
                    const response = await fetch(`/api/validate/${endpoint}?${field}=${encodeURIComponent(value)}`);
                    const data = await response.json();

                    if (response.ok) {
                        errorElement.style.display = 'none';
                        successElement.style.display = 'block';
                        APP.state.validations[field] = true;
                    } else {
                        errorElement.textContent = data.error || `Invalid ${field}`;
                        errorElement.style.display = 'block';
                        successElement.style.display = 'none';
                        APP.state.validations[field] = false;
                    }
                } catch (error) {
                    errorElement.textContent = `Error validating ${field}`;
                    errorElement.style.display = 'block';
                    successElement.style.display = 'none';
                    APP.state.validations[field] = false;
                }

                this.updateSubmitButton();
            },

            updateSubmitButton() {
                const allValid = Object.values(APP.state.validations).every(v => v);
                document.getElementById('submitBtn').disabled = !allValid;
            },

            attachEventListeners() {
                document.getElementById('projectName').addEventListener('input', 
                    APP.utils.debounce(() => this.validateField('projectName', 'projectName'), 500)
                );

                document.getElementById('subdomain').addEventListener('input',
                    APP.utils.debounce(() => this.validateField('subdomain', 'domain'), 500)
                );

                document.getElementById('containerImage').addEventListener('input',
                    APP.utils.debounce(() => this.validateField('containerImage', 'image'), 500)
                );

                document.getElementById('deployForm').addEventListener('submit', async (e) => {
                    e.preventDefault();

                    const formData = {
                        projectName: document.getElementById('projectName').value,
                        subdomain: document.getElementById('subdomain').value,
                        containerImage: document.getElementById('containerImage').value,
                        localPort: document.getElementById('localPort').value
                    };

                    try {
                        const response = await fetch('/api/deploy', {
                            method: 'POST',
                            headers: {
                                'Content-Type': 'application/json',
                                'Authorization': `Bearer ${localStorage.getItem('token')}`
                            },
                            body: JSON.stringify(formData)
                        });

                        if (response.ok) {
                            APP.router.navigate('/apps');
                        }
                    } catch (error) {
                        console.error('Deploy failed:', error);
                    }
                });
            }
        }
    },

    init() {
        this.state.isAuthenticated = this.auth.isAuthenticated();
        this.router.init();
    }
};

document.addEventListener('DOMContentLoaded', () => {
    APP.init();
});
