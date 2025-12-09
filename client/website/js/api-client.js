// API клиент для взаимодействия с C++ бэкендом BongoNews
class BongoNewsAPI {
    constructor() {
        // Определяем базовый URL в зависимости от окружения
        this.baseURL = this._getBaseURL();
        this.defaultHeaders = {
            'Content-Type': 'application/json',
        };
    }

    // Определение базового URL
    _getBaseURL() {
        // Для разработки локально
        if (window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1') {
            return 'http://localhost:8080/api';
        }
        // Для Docker-окружения - бэкенд доступен по имени сервиса
        return '/api'; // Прокси через Nginx
    }

    // Общий метод для выполнения запросов
    async _request(endpoint, options = {}) {
        const url = `${this.baseURL}${endpoint}`;
        const config = {
            headers: this.defaultHeaders,
            ...options
        };

        try {
            const response = await fetch(url, config);
            
            if (!response.ok) {
                // Пробуем получить JSON с ошибкой
                let errorMsg = `HTTP error! status: ${response.status}`;
                try {
                    const errorData = await response.json();
                    if (errorData.error) errorMsg = errorData.error;
                } catch (e) {
                    // Если не JSON, используем текст
                    errorMsg = await response.text() || errorMsg;
                }
                throw new Error(errorMsg);
            }

            // Для DELETE запросов может не быть тела ответа
            if (response.status === 204 || options.method === 'DELETE') {
                return { success: true };
            }

            return await response.json();
        } catch (error) {
            console.error('API request failed:', error);
            throw error;
        }
    }

    // ===== ОСНОВНЫЕ ТАБЛИЦЫ =====
    
    // Посты
    async getPosts() {
        return this._request('/posts');
    }

    async getPost(postId) {
        return this._request(`/posts/${postId}`);
    }

    // Авторы
    async getAuthors() {
        return this._request('/authors');
    }

    // Каналы
    async getChannels() {
        return this._request('/channels');
    }

    // Источники
    async getSources() {
        return this._request('/sources');
    }

    // Теги
    async getTags() {
        return this._request('/tags');
    }

    // Комментарии
    async getComments() {
        return this._request('/comments');
    }

    // ===== РАСШИРЕННЫЕ ПРЕДСТАВЛЕНИЯ =====

    // Полная информация о постах
    async getComprehensivePostInfo() {
        return this._request('/comprehensive_post_info');
    }

    // Расширенная аналитика
    async getExtendedPostAnalytics() {
        return this._request('/extended_post_analytics');
    }

    // Посты с тегами и каналами
    async getPostsWithTagsAndChannels() {
        return this._request('/posts_with_tags_and_channels');
    }

    // Статистика активности каналов
    async getChannelActivityStats() {
        return this._request('/channel_activity_stats');
    }

    // Популярность тегов
    async getTagPopularity() {
        return this._request('/tag_popularity_detailed');
    }

    // Рейтинг постов по популярности
    async getPostsRankedByPopularity() {
        return this._request('/posts_ranked_by_popularity');
    }

    // ===== БИЗНЕС-ЛОГИКА =====

    // Лайк поста
    async likePost(postId) {
        try {
            // Получаем текущий пост
            const postResponse = await this.getPost(postId);
            const post = Array.isArray(postResponse) ? postResponse[0] : postResponse;
            
            if (!post) {
                throw new Error('Post not found');
            }

            // Увеличиваем счетчик лайков
            const currentLikes = post.likes_count || 0;
            const updatedLikes = currentLikes + 1;
            
            // Обновляем пост
            return await this._request(`/posts/${postId}`, {
                method: 'PUT',
                body: JSON.stringify({
                    ...post,
                    likes_count: updatedLikes
                })
            });
        } catch (error) {
            console.error('Like post failed:', error);
            // Если сервер недоступен, имитируем успех для офлайн-работы
            return { success: true, offline: true };
        }
    }

    // Увеличение счетчика комментариев
    async incrementCommentCount(postId) {
        try {
            const post = await this.getPost(postId);
            const currentComments = post.comments_count || 0;
            
            return await this._request(`/posts/${postId}`, {
                method: 'PUT',
                body: JSON.stringify({
                    ...post,
                    comments_count: currentComments + 1
                })
            });
        } catch (error) {
            console.error('Increment comment count failed:', error);
            throw error;
        }
    }

    // Поиск постов
    async searchPosts(query, category = 'all') {
        try {
            // Используем расширенную аналитику для поиска
            const allPosts = await this.getExtendedPostAnalytics();
            
            if (!Array.isArray(allPosts)) {
                console.warn('Expected array from API, got:', typeof allPosts);
                return [];
            }
            
            const searchTerm = query.toLowerCase();
            return allPosts.filter(post => {
                const title = post.title || '';
                const content = post.content_preview || post.content || '';
                const tags = post.tags || '';
                const author = post.author_name || '';
                
                const matchesSearch = 
                    title.toLowerCase().includes(searchTerm) ||
                    content.toLowerCase().includes(searchTerm) ||
                    tags.toLowerCase().includes(searchTerm) ||
                    author.toLowerCase().includes(searchTerm);
                
                const matchesCategory = category === 'all' || 
                    this._determineCategory(post) === category;
                
                return matchesSearch && matchesCategory;
            });
        } catch (error) {
            console.error('Search failed:', error);
            // Возвращаем пустой массив при ошибке
            return [];
        }
    }

    // Получение популярных постов
    async getPopularPosts(limit = 10) {
        try {
            const rankedPosts = await this.getPostsRankedByPopularity();
            
            if (!Array.isArray(rankedPosts)) {
                console.warn('Expected array from popularity endpoint');
                return [];
            }
            
            return rankedPosts
                .sort((a, b) => (b.likes_count || 0) - (a.likes_count || 0))
                .slice(0, limit);
        } catch (error) {
            console.error('Error getting popular posts:', error);
            // Fallback к обычным постам
            try {
                const allPosts = await this.getExtendedPostAnalytics();
                if (Array.isArray(allPosts)) {
                    return allPosts
                        .sort((a, b) => (b.likes_count || 0) - (a.likes_count || 0))
                        .slice(0, limit);
                }
            } catch (e) {
                console.error('Fallback also failed:', e);
            }
            return [];
        }
    }

    // Получение недавних постов
    async getRecentPosts(limit = 10) {
        try {
            const allPosts = await this.getExtendedPostAnalytics();
            
            if (!Array.isArray(allPosts)) {
                return [];
            }
            
            return allPosts
                .sort((a, b) => {
                    const dateA = a.created_at ? new Date(a.created_at) : new Date(0);
                    const dateB = b.created_at ? new Date(b.created_at) : new Date(0);
                    return dateB - dateA;
                })
                .slice(0, limit);
        } catch (error) {
            console.error('Get recent posts failed:', error);
            return [];
        }
    }

    // ===== ВСПОМОГАТЕЛЬНЫЕ МЕТОДЫ =====

    // Определение категории на основе тегов и темы
    _determineCategory(post) {
        const categoryMap = {
            'технологии': 'technology',
            'технология': 'technology',
            'technology': 'technology',
            'тех': 'technology',
            'программирование': 'technology',
            'programming': 'technology',
            'код': 'technology',
            'ai': 'technology',
            'ии': 'technology',
            'искуственный интеллект': 'technology',
            'наука': 'science',
            'science': 'science',
            'scientific': 'science',
            'исследование': 'science',
            'исследования': 'science',
            'research': 'science',
            'культура': 'culture',
            'culture': 'culture',
            'искусство': 'culture',
            'art': 'culture',
            'дизайн': 'culture',
            'design': 'culture',
            'музыка': 'culture',
            'music': 'culture'
        };

        const sourceTopic = (post.source_topic || '').toLowerCase();
        const tags = (post.tags || '').toLowerCase();
        const channelName = (post.channel_name || '').toLowerCase();

        // Проверяем тему источника
        for (const [key, value] of Object.entries(categoryMap)) {
            if (sourceTopic.includes(key)) {
                return value;
            }
        }

        // Проверяем теги
        for (const [key, value] of Object.entries(categoryMap)) {
            if (tags.includes(key)) {
                return value;
            }
        }

        // Проверяем название канала
        for (const [key, value] of Object.entries(categoryMap)) {
            if (channelName.includes(key)) {
                return value;
            }
        }

        return 'technology'; // Категория по умолчанию
    }
}

// Глобальный интерфейс для интеграции с существующим фронтендом
window.BongoNewsAPI = {
    _api: new BongoNewsAPI(),

    // Инициализация
    init() {
        console.log('BongoNews API клиент инициализирован. Base URL:', this._api.baseURL);
        return this._api;
    },

    // Загрузка популярных постов для главной страницы
    async loadPopularPosts(limit = 8) {
        try {
            console.log('Loading popular posts...');
            const posts = await this._api.getPopularPosts(limit);
            console.log(`Loaded ${posts.length} popular posts`);
            return this._transformPostsData(posts);
        } catch (error) {
            console.error('Failed to load popular posts:', error);
            return this._getFallbackData();
        }
    },

    // Загрузка недавних постов
    async loadRecentPosts(limit = 8) {
        try {
            console.log('Loading recent posts...');
            const posts = await this._api.getRecentPosts(limit);
            console.log(`Loaded ${posts.length} recent posts`);
            return this._transformPostsData(posts);
        } catch (error) {
            console.error('Failed to load recent posts:', error);
            return this._getFallbackData();
        }
    },

    // Поиск постов
    async searchPosts(query, category = 'all') {
        try {
            console.log(`Searching for "${query}" in category "${category}"...`);
            const posts = await this._api.searchPosts(query, category);
            console.log(`Found ${posts.length} results`);
            return this._transformPostsData(posts);
        } catch (error) {
            console.error('Search failed:', error);
            return [];
        }
    },

    // Лайк поста
    async likePost(postId) {
        try {
            console.log('Liking post:', postId);
            const result = await this._api.likePost(postId);
            console.log('Like result:', result);
            
            if (result.offline) {
                console.log('Server offline, stored like locally');
                // Сохраняем лайк в localStorage для синхронизации позже
                this._storePendingLike(postId);
                return true;
            }
            
            return true;
        } catch (error) {
            console.error('Like failed:', error);
            return false;
        }
    },

    // Хранение лайков офлайн
    _storePendingLike(postId) {
        const pendingLikes = JSON.parse(localStorage.getItem('pendingLikes') || '[]');
        if (!pendingLikes.includes(postId)) {
            pendingLikes.push(postId);
            localStorage.setItem('pendingLikes', JSON.stringify(pendingLikes));
        }
    },

    // Преобразование данных из API в формат фронтенда
    _transformPostsData(posts) {
        if (!Array.isArray(posts)) {
            console.warn('Expected array of posts, got:', typeof posts);
            return [];
        }
        
        return posts.map((post, index) => ({
            id: post.post_id || post.id || `fallback-${index}`,
            title: post.title || 'Без названия',
            excerpt: this._generateExcerpt(post.content_preview || post.content || ''),
            image: this._getPostImage(post, index),
            date: this._formatDate(post.created_at || post.published_at),
            category: this._determineCategory(post),
            tags: this._extractTags(post),
            likes: post.likes_count || 0,
            comments: post.comments_count || 0,
            author: post.author_name || 'Неизвестный автор',
            channel: post.channel_name || 'Без канала',
            source: post.source_name || 'Без источника'
        }));
    },

    // Генерация краткого описания
    _generateExcerpt(content) {
        if (!content) return 'Описание недоступно';
        const plainText = content.replace(/<[^>]*>/g, '');
        return plainText.length > 150 
            ? plainText.substring(0, 150) + '...' 
            : plainText;
    },

    // Получение изображения поста
    _getPostImage(post, index) {
        // Проверяем, есть ли URL медиа в данных
        if (post.media_url && post.media_url !== '') {
            return post.media_url;
        }
        
        // Используем заглушки из Unsplash с разными категориями
        const categories = ['tech', 'science', 'nature', 'business', 'people'];
        const category = categories[index % categories.length];
        return `https://images.unsplash.com/photo-${1500000000000 + index}?ixlib=rb-1.2.1&auto=format&fit=crop&w=600&h=400&q=80`;
    },

    // Форматирование даты
    _formatDate(dateString) {
        if (!dateString) return 'Дата не указана';
        
        try {
            const date = new Date(dateString);
            
            // Проверяем, что дата валидна
            if (isNaN(date.getTime())) {
                return 'Неверная дата';
            }
            
            const now = new Date();
            const diffMs = now - date;
            const diffHours = Math.floor(diffMs / (1000 * 60 * 60));
            const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));
            
            // Если меньше 24 часов назад
            if (diffHours < 24) {
                if (diffHours < 1) {
                    const diffMins = Math.floor(diffMs / (1000 * 60));
                    return diffMins === 0 ? 'только что' : `${diffMins} мин. назад`;
                }
                return `${diffHours} ч. назад`;
            }
            
            // Если меньше 7 дней назад
            if (diffDays < 7) {
                return `${diffDays} дн. назад`;
            }
            
            // Форматируем дату
            const options = { 
                day: 'numeric', 
                month: 'long', 
                year: date.getFullYear() !== now.getFullYear() ? 'numeric' : undefined
            };
            
            return date.toLocaleDateString('ru-RU', options);
        } catch (error) {
            console.error('Date formatting error:', error);
            return 'Дата не указана';
        }
    },

    // Определение категории (адаптер для фронтенда)
    _determineCategory(post) {
        return this._api._determineCategory(post);
    },

    // Извлечение тегов
    _extractTags(post) {
        let tags = [];
        
        // Если теги есть в виде строки
        if (post.tags && typeof post.tags === 'string') {
            tags = post.tags.split(',').map(tag => tag.trim()).filter(tag => tag);
        }
        
        // Если тегов нет, добавляем базовые на основе категории
        if (tags.length === 0) {
            const baseTags = {
                'technology': ['технологии', 'инновации', 'программирование'],
                'science': ['наука', 'исследования', 'открытия'],
                'culture': ['культура', 'искусство', 'творчество']
            };

            const category = this._determineCategory(post);
            tags = baseTags[category] || ['новости'];
        }
        
        return tags.slice(0, 3); // Ограничиваем до 3 тегов
    },

    // Запасные данные при ошибке API
    _getFallbackData() {
        console.warn('Using fallback data - API might be unavailable');
        
        // Создаем демо-данные для отображения при ошибке API
        const fallbackPosts = [
            {
                id: 'fallback-1',
                title: 'Новые технологии в веб-разработке',
                excerpt: 'Исследование современных тенденций в создании веб-приложений',
                image: 'https://images.unsplash.com/photo-1555066931-4365d14bab8c?ixlib=rb-1.2.1&auto=format&fit=crop&w=600&h=400&q=80',
                date: 'вчера',
                category: 'technology',
                tags: ['технологии', 'веб-разработка', 'тренды'],
                likes: 42,
                comments: 8,
                author: 'Иван Петров',
                channel: 'Техноблог',
                source: 'Medium'
            },
            {
                id: 'fallback-2',
                title: 'Искусственный интеллект и медицина',
                excerpt: 'Как нейросети помогают в диагностике заболеваний',
                image: 'https://images.unsplash.com/photo-1559757148-5c350d0d3c56?ixlib=rb-1.2.1&auto=format&fit=crop&w=600&h=400&q=80',
                date: '3 дня назад',
                category: 'science',
                tags: ['ии', 'медицина', 'нейросети'],
                likes: 89,
                comments: 15,
                author: 'Анна Сидорова',
                channel: 'Научные открытия',
                source: 'ResearchGate'
            }
        ];
        
        return fallbackPosts;
    }
};

// Автоматическая инициализация при загрузке страницы
document.addEventListener('DOMContentLoaded', function() {
    window.BongoNewsAPI.init();
    console.log('BongoNews API готов к использованию');
    
    // Пробуем синхронизировать pending likes
    const pendingLikes = JSON.parse(localStorage.getItem('pendingLikes') || '[]');
    if (pendingLikes.length > 0) {
        console.log(`Есть ${pendingLikes.length} лайков для синхронизации`);
    }
});