#!/bin/bash

# Цвета для вывода
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Переменная для хранения метода аутентификации
AUTH_METHOD=""

# Функция для выполнения MongoDB команд с разными вариантами аутентификации
run_mongo() {
    local command="$1"
    
    if [ -z "$AUTH_METHOD" ]; then
        detect_auth_method
    fi
    
    case $AUTH_METHOD in
        "admin")
            # Используем root/admin пользователя
            docker exec mongodb mongosh --quiet \
                --eval "$command" \
                admin \
                -u admin \
                -p mongopass \
                --authenticationDatabase admin
            ;;
        "app")
            # Используем app пользователя
            docker exec mongodb mongosh --quiet \
                --eval "$command" \
                news_aggregator \
                -u news_app \
                -p app_password \
                --authenticationDatabase news_aggregator
            ;;
        "none")
            # Без аутентификации
            docker exec mongodb mongosh --quiet --eval "$command"
            ;;
        *)
            echo -e "${RED}❌ Не удалось определить метод аутентификации${NC}"
            return 1
            ;;
    esac
}

# Функция для определения метода аутентификации
detect_auth_method() {
    echo -e "${CYAN}Определение метода аутентификации...${NC}"
    
    # Попробуем admin пользователя
    if docker exec mongodb mongosh --quiet \
        --eval "print('✅ Admin auth OK')" \
        admin \
        -u admin \
        -p mongopass \
        --authenticationDatabase admin 2>/dev/null; then
        AUTH_METHOD="admin"
        echo -e "${GREEN}✅ Используется аутентификация: admin/mongopass${NC}"
        return 0
    fi
    
    # Попробуем app пользователя
    if docker exec mongodb mongosh --quiet \
        --eval "print('✅ App auth OK')" \
        news_aggregator \
        -u news_app \
        -p app_password \
        --authenticationDatabase news_aggregator 2>/dev/null; then
        AUTH_METHOD="app"
        echo -e "${GREEN}✅ Используется аутентификация: news_app/app_password${NC}"
        return 0
    fi
    
    # Попробуем без аутентификации
    if docker exec mongodb mongosh --quiet \
        --eval "print('✅ No auth OK')" 2>/dev/null; then
        AUTH_METHOD="none"
        echo -e "${GREEN}✅ Используется подключение без аутентификации${NC}"
        return 0
    fi
    
    echo -e "${RED}❌ Не удалось подключиться к MongoDB${NC}"
    echo -e "${YELLOW}Возможные причины:${NC}"
    echo -e "1. Контейнер MongoDB не запущен"
    echo -e "2. Неправильные учетные данные"
    echo -e "3. MongoDB еще не инициализирована"
    return 1
}

# Функция для форматирования вывода
format_output() {
    echo -e "${BLUE}=============================================${NC}"
    echo -e "${CYAN}$1${NC}"
    echo -e "${BLUE}=============================================${NC}"
}

# Главное меню
show_menu() {
    clear
    echo -e "${GREEN}=================================${NC}"
    echo -e "${YELLOW}   MongoDB Monitoring Dashboard${NC}"
    echo -e "${GREEN}=================================${NC}"
    echo ""
    echo -e "Метод аутентификации: ${CYAN}${AUTH_METHOD:-не определен}${NC}"
    echo ""
    echo -e "1.  ${CYAN}Общие метрики сервера${NC}"
    echo -e "2.  ${CYAN}Статистика соединений${NC}"
    echo -e "3.  ${CYAN}Использование памяти${NC}"
    echo -e "4.  ${CYAN}Операции (операции/сек)${NC}"
    echo -e "5.  ${CYAN}Статистика коллекций${NC}"
    echo -e "6.  ${CYAN}Использование индексов${NC}"
    echo -e "7.  ${CYAN}Активные операции${NC}"
    echo -e "8.  ${CYAN}Медленные запросы${NC}"
    echo -e "9.  ${CYAN}Статус репликации${NC}"
    echo -e "10. ${CYAN}Метрики для Prometheus${NC}"
    echo -e "11. ${CYAN}Полный отчет${NC}"
    echo -e "12. ${CYAN}Мониторинг в реальном времени${NC}"
    echo -e "13. ${CYAN}Проверить/сменить подключение${NC}"
    echo -e "0.  ${RED}Выход${NC}"
    echo ""
    echo -n "Выберите опцию [0-13]: "
}

# 1. Общие метрики сервера
show_general_metrics() {
    format_output "ОБЩИЕ МЕТРИКИ СЕРВЕРА"
    
    # Для admin аутентификации используем admin команды
    if [ "$AUTH_METHOD" = "admin" ]; then
        run_mongo "
        const status = db.adminCommand({serverStatus: 1});
        
        console.log('Версия MongoDB: ' + status.version);
        console.log('Аптайм: ' + status.uptime + ' секунд (' + Math.round(status.uptime/60) + ' минут)');
        console.log('Хост: ' + status.host);
        console.log('Процесс ID: ' + status.pid);
        console.log('Текущее время: ' + new Date(status.localTime));
        
        // Проверка режима
        if (status.storageEngine && status.storageEngine.name) {
            console.log('Движок хранилища: ' + status.storageEngine.name);
        }
        
        // Проверка журналирования
        if (status.storageEngine && status.storageEngine.supportsCommittedReads !== undefined) {
            console.log('Поддержка транзакций: ' + (status.storageEngine.supportsCommittedReads ? 'Да' : 'Нет'));
        }
        "
    else
        # Для других методов
        run_mongo "
        const status = db.serverStatus();
        
        console.log('Версия MongoDB: ' + status.version);
        console.log('Аптайм: ' + status.uptime + ' секунд (' + Math.round(status.uptime/60) + ' минут)');
        console.log('Хост: ' + status.host);
        console.log('Процесс ID: ' + status.pid);
        console.log('Текущее время: ' + new Date(status.localTime));
        
        // Проверка режима
        if (status.storageEngine && status.storageEngine.name) {
            console.log('Движок хранилища: ' + status.storageEngine.name);
        }
        
        // Проверка журналирования
        if (status.storageEngine && status.storageEngine.supportsCommittedReads !== undefined) {
            console.log('Поддержка транзакций: ' + (status.storageEngine.supportsCommittedReads ? 'Да' : 'Нет'));
        }
        "
    fi
    
    echo -e "\n${GREEN}Нажмите Enter для продолжения...${NC}"
    read
}

# 2. Статистика соединений
show_connections() {
    format_output "СТАТИСТИКА СОЕДИНЕНИЙ"
    
    run_mongo "
    const status = db.serverStatus();
    const conn = status.connections || {};
    
    console.log('Текущие соединения: ' + conn.current);
    console.log('Доступные соединения: ' + conn.available);
    console.log('Всего создано: ' + conn.totalCreated);
    
    const usagePercent = ((conn.current / conn.available) * 100).toFixed(1);
    console.log('Использование: ' + usagePercent + '%');
    
    if (conn.active) {
        console.log('Активные: ' + conn.active);
    }
    if (conn.threaded) {
        console.log('Потоковых: ' + conn.threaded);
    }
    
    // Цветовая индикация
    if (usagePercent > 80) {
        console.log('⚠️  ВНИМАНИЕ: Высокое использование соединений!');
    } else if (usagePercent > 60) {
        console.log('ℹ️  ИНФО: Умеренное использование соединений');
    } else {
        console.log('✅ ОК: Нормальное использование соединений');
    }
    "
    
    echo -e "\n${GREEN}Нажмите Enter для продолжения...${NC}"
    read
}

# 3. Использование памяти
show_memory_usage() {
    format_output "ИСПОЛЬЗОВАНИЕ ПАМЯТИ"
    
    run_mongo "
    try {
        const status = db.serverStatus();
        const mem = status.mem || {};
        
        console.log('📊 Статистика памяти MongoDB:');
        
        // 1. ОСНОВНАЯ ПАМЯТЬ (УЖЕ В МЕГАБАЙТАХ!)
        const residentMB = mem.resident || 0;
        console.log('🏠 Резидентная память (RAM): ' + residentMB + ' MB');
        
        const virtualMB = mem.virtual || 0;
        console.log('💽 Виртуальная память: ' + virtualMB + ' MB');
        
        // 2. MAPPED ПАМЯТЬ (ТОЖЕ В МЕГАБАЙТАХ!)
        if (mem.mapped) {
            console.log('🗺️  Mapped память: ' + mem.mapped + ' MB');
        }
        
        if (mem.mappedWithJournal) {
            console.log('📝 Mapped с журналом: ' + mem.mappedWithJournal + ' MB');
        }
        
        // 3. SUPPORTED - ЭТО BOOLEAN, НЕ РАЗМЕР!
        console.log('⚡ Поддерживаемая память: ' + (mem.supported ? 'Да' : 'Нет'));
        
        // 4. АРХИТЕКТУРА
        if (mem.bits) {
            console.log('🔢 Архитектура: ' + mem.bits + '-bit');
        }
        
        // 5. РАСЧЕТ ИСПОЛЬЗОВАНИЯ ОТНОСИТЕЛЬНО ВСЕЙ RAM ХОСТА
        if (status.hostInfo && status.hostInfo.system && status.hostInfo.system.memSizeMB) {
            const totalRAM = status.hostInfo.system.memSizeMB;
            console.log('💻 Всего RAM на хосте: ' + totalRAM + ' MB');
            
            if (residentMB > 0 && totalRAM > 0) {
                const usagePercent = ((residentMB / totalRAM) * 100).toFixed(1);
                console.log('📈 Использование от общей RAM: ' + usagePercent + '%');
                
                if (usagePercent > 90) {
                    console.log('⚠️  ВНИМАНИЕ: Высокое использование памяти!');
                } else if (usagePercent > 70) {
                    console.log('ℹ️  ИНФО: Умеренное использование памяти');
                } else {
                    console.log('✅ ОК: Нормальное использование памяти');
                }
            }
        }
        
        // 6. WIREDTIGER КЭШ (В БАЙТАХ - ДЕЛИМ!)
        console.log('\n🔍 Дополнительные метрики:');
        
        if (status.wiredTiger && status.wiredTiger.cache) {
            const cache = status.wiredTiger.cache;
            console.log('🐯 WiredTiger кэш:');
            
            // ВНИМАНИЕ: эти значения в БАЙТАХ!
            const cacheCurrent = cache['bytes currently in the cache'] || 0;
            const cacheCurrentMB = Math.round(cacheCurrent / 1024 / 1024);
            
            const cacheMax = cache['maximum bytes configured'] || 0;
            const cacheMaxMB = Math.round(cacheMax / 1024 / 1024);
            
            console.log('   Размер: ' + cacheCurrentMB + ' MB');
            console.log('   Макс. размер: ' + cacheMaxMB + ' MB');
            
            if (cacheCurrent > 0 && cacheMax > 0) {
                const cacheUsage = ((cacheCurrent / cacheMax) * 100).toFixed(1);
                console.log('   Использование кэша: ' + cacheUsage + '%');
            }
        }
        
    } catch(e) {
        console.log('❌ Ошибка при получении статистики памяти: ' + e.message);
    }
    "
    
    echo -e "\n🐳 Для сравнения - Docker Stats:"
    docker stats mongodb --no-stream --format "table {{.Name}}\t{{.MemUsage}}\t{{.MemPerc}}" 2>/dev/null || echo "  Docker stats недоступен"
    
    echo -e "\n${GREEN}Нажмите Enter для продолжения...${NC}"
    read
}

# 4. Операции
show_operations() {
    format_output "СТАТИСТИКА ОПЕРАЦИЙ"
    
    run_mongo "
    const status = db.serverStatus();
    const ops = status.opcounters || {};
    
    console.log('Операции с момента запуска:');
    console.log('  Вставки: ' + (ops.insert || 0));
    console.log('  Запросы: ' + (ops.query || 0));
    console.log('  Обновления: ' + (ops.update || 0));
    console.log('  Удаления: ' + (ops.delete || 0));
    console.log('  GetMore: ' + (ops.getmore || 0));
    console.log('  Команды: ' + (ops.command || 0));
    
    // Операции в секунду (приблизительно)
    const uptime = status.uptime;
    if (uptime > 0) {
        console.log('\nСреднее в секунду:');
        console.log('  Вставки: ' + (ops.insert / uptime).toFixed(2));
        console.log('  Запросы: ' + (ops.query / uptime).toFixed(2));
        console.log('  Обновления: ' + (ops.update / uptime).toFixed(2));
        console.log('  Удаления: ' + (ops.delete / uptime).toFixed(2));
    }
    "
    
    echo -e "\n${GREEN}Нажмите Enter для продолжения...${NC}"
    read
}

# 5. Статистика коллекций
show_collection_stats() {
    format_output "СТАТИСТИКА КОЛЛЕКЦИЙ"
    
    run_mongo "
    try {
        const db = db.getSiblingDB('news_aggregator');
        const collections = db.getCollectionNames();
        
        console.log('База данных: news_aggregator');
        console.log('Коллекций: ' + collections.length);
        console.log('');
        
        console.log('Название'.padEnd(25) + 'Документы'.padStart(10) + 'Размер'.padStart(12) + 'Индексы'.padStart(10));
        console.log('-'.repeat(57));
        
        let totalDocs = 0;
        let totalSize = 0;
        let totalIndexes = 0;
        
        collections.forEach(collName => {
            try {
                const stats = db[collName].stats();
                const docs = stats.count || 0;
                const size = Math.round((stats.size || 0) / 1024);
                const indexes = stats.nindexes || 0;
                
                console.log(
                    collName.padEnd(25) +
                    docs.toString().padStart(10) +
                    (size + ' KB').padStart(12) +
                    indexes.toString().padStart(10)
                );
                
                totalDocs += docs;
                totalSize += stats.size || 0;
                totalIndexes += indexes;
            } catch(e) {
                console.log(collName.padEnd(25) + 'ОШИБКА'.padStart(30));
            }
        });
        
        console.log('-'.repeat(57));
        console.log(
            'ИТОГО:'.padEnd(25) +
            totalDocs.toString().padStart(10) +
            (Math.round(totalSize / 1024) + ' KB').padStart(12) +
            totalIndexes.toString().padStart(10)
        );
    } catch(e) {
        console.log('Ошибка доступа к базе news_aggregator: ' + e.message);
        console.log('Текущая база: ' + db.getName());
        console.log('Доступные базы: ' + JSON.stringify(db.getMongo().getDBs()));
    }
    "
    
    echo -e "\n${GREEN}Нажмите Enter для продолжения...${NC}"
    read
}

# 6. Использование индексов
show_index_usage() {
    format_output "ИСПОЛЬЗОВАНИЕ ИНДЕКСОВ"
    
    run_mongo "
    try {
        const db = db.getSiblingDB('news_aggregator');
        
        // Получаем статистику использования индексов
        const indexStats = {};
        
        // Для каждой коллекции
        db.getCollectionNames().forEach(collName => {
            const coll = db.getCollection(collName);
            const stats = coll.stats();
            
            if (stats.nindexes > 0) {
                console.log('\nКоллекция: ' + collName);
                console.log('  Индексов: ' + stats.nindexes);
                console.log('  Размер индексов: ' + Math.round(stats.totalIndexSize / 1024 / 1024 * 100) / 100 + ' MB');
                
                // Показываем отдельные индексы
                const indexes = coll.getIndexes();
                indexes.forEach((idx, i) => {
                    console.log('  ' + (i+1) + '. ' + idx.name + ':');
                    console.log('     Поля: ' + JSON.stringify(idx.key));
                    if (idx.unique) console.log('     Уникальный: Да');
                    if (idx.sparse) console.log('     Разреженный: Да');
                    if (idx.expireAfterSeconds) console.log('     TTL: ' + idx.expireAfterSeconds + ' секунд');
                });
            }
        });
    } catch(e) {
        console.log('Ошибка: ' + e.message);
    }
    "
    
    echo -e "\n${GREEN}Нажмите Enter для продолжения...${NC}"
    read
}

# 7. Активные операции
show_active_operations() {
    format_output "АКТИВНЫЕ ОПЕРАЦИИ"
    
    run_mongo "
    try {
        const ops = db.currentOp();
        
        if (ops.inprog && ops.inprog.length > 0) {
            console.log('Активных операций: ' + ops.inprog.length);
            console.log('');
            
            ops.inprog.forEach((op, index) => {
                console.log('Операция #' + (index + 1) + ':');
                console.log('  ID: ' + op.opid);
                console.log('  Тип: ' + op.op);
                console.log('  База/Коллекция: ' + (op.ns || 'N/A'));
                console.log('  Выполняется: ' + op.secs_running + ' секунд');
                console.log('  Состояние: ' + (op.msg || 'N/A'));
                
                if (op.command) {
                    console.log('  Команда: ' + JSON.stringify(op.command).substring(0, 100) + '...');
                }
                console.log('');
            });
        } else {
            console.log('✅ Нет активных операций');
        }
    } catch(e) {
        console.log('Ошибка получения активных операций: ' + e.message);
    }
    "
    
    echo -e "\n${GREEN}Нажмите Enter для продолжения...${NC}"
    read
}

# 8. Медленные запросы
show_slow_queries() {
    format_output "МЕДЛЕННЫЕ ЗАПРОСЫ (более 100ms)"
    
    run_mongo "
    try {
        const db = db.getSiblingDB('news_aggregator');
        
        console.log('Проверка профилировщика...');
        
        // Включаем профилировщик если выключен
        const profilerStatus = db.getProfilingStatus();
        if (profilerStatus.was == 0) {
            console.log('ℹ️  Профилировщик выключен. Включаем на время проверки...');
            db.setProfilingLevel(1, 100); // Включаем для запросов > 100ms
        }
        
        // Получаем медленные запросы
        const slowQueries = db.system.profile
            .find({ millis: { \$gt: 100 } })
            .sort({ ts: -1 })
            .limit(10)
            .toArray();
        
        if (slowQueries.length > 0) {
            console.log('Найдено медленных запросов: ' + slowQueries.length);
            console.log('');
            
            slowQueries.forEach((query, index) => {
                console.log('Медленный запрос #' + (index + 1) + ':');
                console.log('  Время: ' + query.ts);
                console.log('  Длительность: ' + query.millis + ' ms');
                console.log('  Операция: ' + query.op);
                console.log('  Коллекция: ' + query.ns);
                
                if (query.command) {
                    console.log('  Команда: ' + JSON.stringify(query.command).substring(0, 150));
                }
                
                if (query.planSummary) {
                    console.log('  План: ' + query.planSummary);
                }
                
                console.log('');
            });
        } else {
            console.log('✅ Медленных запросов не найдено');
        }
        
        // Возвращаем исходный уровень профилирования
        db.setProfilingLevel(profilerStatus.was, profilerStatus.slowms);
    } catch(e) {
        console.log('Ошибка при проверке медленных запросов: ' + e.message);
    }
    "
    
    echo -e "\n${GREEN}Нажмите Enter для продолжения...${NC}"
    read
}

# 9. Статус репликации
show_replication_status() {
    format_output "СТАТУС РЕПЛИКАЦИИ"
    
    run_mongo "
    try {
        const status = rs.status();
        
        console.log('Набор реплик: ' + status.set);
        console.log('Дата: ' + status.date);
        console.log('');
        
        console.log('Члены набора:');
        console.log('№  Имя'.padEnd(25) + 'Статус'.padEnd(20) + 'Здоровье'.padEnd(10) + 'Lag');
        console.log('-'.repeat(65));
        
        status.members.forEach((member, index) => {
            const lag = member.optimeDate ? 
                Math.round((new Date() - member.optimeDate) / 1000) : 'N/A';
            
            console.log(
                (index+1).toString().padEnd(3) +
                (member.name || 'N/A').padEnd(25) +
                (member.stateStr || 'N/A').padEnd(20) +
                (member.health || 0).toString().padEnd(10) +
                (lag + 's')
            );
        });
        
    } catch(e) {
        console.log('Репликация не настроена: ' + e.message);
        console.log('Текущий режим: standalone');
    }
    "
    
    echo -e "\n${GREEN}Нажмите Enter для продолжения...${NC}"
    read
}

# 10. Метрики для Prometheus
show_prometheus_metrics() {
    format_output "МЕТРИКИ В ФОРМАТЕ PROMETHEUS"
    
    run_mongo "
    try {
        const status = db.serverStatus();
        const dbStats = db.getSiblingDB('news_aggregator').stats();
        
        // Формат Prometheus
        console.log('# HELP mongodb_up Whether MongoDB is up');
        console.log('# TYPE mongodb_up gauge');
        console.log('mongodb_up 1');
        
        console.log('# HELP mongodb_version_info MongoDB version info');
        console.log('# TYPE mongodb_version_info gauge');
        console.log('mongodb_version_info{version=\"' + status.version + '\"} 1');
        
        console.log('# HELP mongodb_connections_current Current connections');
        console.log('# TYPE mongodb_connections_current gauge');
        console.log('mongodb_connections_current ' + (status.connections.current || 0));
        
        console.log('# HELP mongodb_connections_available Available connections');
        console.log('# TYPE mongodb_connections_available gauge');
        console.log('mongodb_connections_available ' + (status.connections.available || 0));
        
        console.log('# HELP mongodb_memory_resident_megabytes Resident memory in megabytes');
        console.log('# TYPE mongodb_memory_resident_megabytes gauge');
        console.log('mongodb_memory_resident_megabytes ' + (status.mem.resident || 0));
        
        console.log('# HELP mongodb_memory_virtual_megabytes Virtual memory in megabytes');
        console.log('# TYPE mongodb_memory_virtual_megabytes gauge');
        console.log('mongodb_memory_virtual_megabytes ' + (status.mem.virtual || 0));
        
        console.log('# HELP mongodb_operations_total Total operations since startup');
        console.log('# TYPE mongodb_operations_total counter');
        console.log('mongodb_operations_total{type=\"insert\"} ' + (status.opcounters.insert || 0));
        console.log('mongodb_operations_total{type=\"query\"} ' + (status.opcounters.query || 0));
        console.log('mongodb_operations_total{type=\"update\"} ' + (status.opcounters.update || 0));
        console.log('mongodb_operations_total{type=\"delete\"} ' + (status.opcounters.delete || 0));
        
        console.log('# HELP mongodb_documents_total Total documents in database');
        console.log('# TYPE mongodb_documents_total gauge');
        console.log('mongodb_documents_total ' + (dbStats.objects || 0));
        
        console.log('# HELP mongodb_database_size_bytes Database size in bytes');
        console.log('# TYPE mongodb_database_size_bytes gauge');
        console.log('mongodb_database_size_bytes ' + (dbStats.dataSize || 0));
        
        console.log('# HELP mongodb_index_size_bytes Total index size in bytes');
        console.log('# TYPE mongodb_index_size_bytes gauge');
        console.log('mongodb_index_size_bytes ' + (dbStats.indexSize || 0));
    } catch(e) {
        console.log('# HELP mongodb_up Whether MongoDB is up');
        console.log('# TYPE mongodb_up gauge');
        console.log('mongodb_up 0');
        console.log('# ERROR ' + e.message);
    }
    "
    
    echo -e "\n${YELLOW}Эти метрики можно сохранить в файл и использовать с Prometheus:${NC}"
    echo -e "  ./mango.sh 10 > mongodb_metrics.prom"
    echo -e "\n${GREEN}Нажмите Enter для продолжения...${NC}"
    read
}

# 11. Полный отчет
show_full_report() {
    format_output "ПОЛНЫЙ ОТЧЕТ О СОСТОЯНИИ MONGODB"
    
    echo -e "${CYAN}Собираем данные...${NC}\n"
    
    # Собираем все метрики
    show_general_metrics
    show_connections
    show_memory_usage
    show_operations
    show_collection_stats
    show_index_usage
    show_active_operations
    show_slow_queries
    show_replication_status
    
    echo -e "${GREEN}✅ Полный отчет завершен${NC}"
    echo -e "\n${GREEN}Нажмите Enter для возврата в меню...${NC}"
    read
}

# 12. Мониторинг в реальном времени
show_realtime_monitor() {
    format_output "МОНИТОРИНГ В РЕАЛЬНОМ ВРЕМЕНИ"
    
    echo -e "${YELLOW}Нажмите Ctrl+C для выхода${NC}\n"
    
    # Сохраняем начальные счетчики операций
    PREV_STATS=$(run_mongo "
        const status = db.serverStatus();
        console.log(JSON.stringify({
            ops: status.opcounters,
            time: Date.now()
        }));
    ")
    
    trap 'echo -e "\n${GREEN}Мониторинг остановлен${NC}"; return' INT
    
    while true; do
        clear
        
        # Текущее время
        echo -e "${BLUE}$(date '+%Y-%m-%d %H:%M:%S')${NC}"
        echo -e "${CYAN}MongoDB Realtime Monitor${NC}"
        echo -e "${BLUE}═══════════════════════════${NC}\n"
        
        # Получаем текущий статус
        CURRENT_STATUS=$(run_mongo "
            const status = db.serverStatus();
            const conn = status.connections;
            const mem = status.mem;
            
            console.log('Соединения: ' + conn.current + '/' + conn.available);
            console.log('Память: ' + mem.resident + ' MB');
            console.log('Аптайм: ' + Math.round(status.uptime/60) + ' мин');
        ")
        
        echo -e "$CURRENT_STATUS"
        
        # Получаем текущие операции для расчета в секунду
        CURRENT_STATS=$(run_mongo "
            const status = db.serverStatus();
            console.log(JSON.stringify({
                ops: status.opcounters,
                time: Date.now()
            }));
        ")
        
        if [ ! -z "$PREV_STATS" ] && [ ! -z "$CURRENT_STATS" ]; then
            # Используем Python для парсинга JSON и расчета
            OPS_DIFF=$(python3 -c "
import json, sys
try:
    prev = json.loads('$PREV_STATS')
    curr = json.loads('$CURRENT_STATS')
    
    time_diff = (curr['time'] - prev['time']) / 1000.0  # секунды
    
    if time_diff > 0:
        ops = curr['ops']
        prev_ops = prev['ops']
        
        print('\\n📊 Операции/сек:')
        for key in ['insert', 'query', 'update', 'delete', 'command']:
            if key in ops and key in prev_ops:
                diff = ops[key] - prev_ops[key]
                ops_per_sec = diff / time_diff
                print(f'  {key}: {ops_per_sec:.2f}/s')
except Exception as e:
    print('  (расчет недоступен)')
            ")
            
            echo -e "$OPS_DIFF"
        fi
        
        PREV_STATS="$CURRENT_STATS"
        
        echo -e "\n${GREEN}Обновление через 2 секунды...${NC}"
        sleep 2
    done
}

# 13. Проверка подключения
show_connection_test() {
    format_output "ПРОВЕРКА ПОДКЛЮЧЕНИЯ К MONGODB"
    
    # Сбрасываем текущий метод
    AUTH_METHOD=""
    
    if detect_auth_method; then
        echo -e "\n${GREEN}✅ Подключение успешно установлено${NC}"
        echo -e "Используется метод: ${CYAN}${AUTH_METHOD}${NC}"
        
        # Проверяем доступные базы
        echo -e "\n${YELLOW}Проверка доступных баз данных:${NC}"
        run_mongo "print('Доступные базы: ' + JSON.stringify(db.getMongo().getDBs()))"
    else
        echo -e "\n${RED}❌ Не удалось подключиться к MongoDB${NC}"
        echo -e "\n${YELLOW}Попробуйте:${NC}"
        echo -e "1. Проверить, запущен ли контейнер: docker ps | grep mongo"
        echo -e "2. Проверить логи: docker logs mongodb"
        echo -e "3. Подключиться вручную: docker exec -it mongodb mongosh"
    fi
    
    echo -e "\n${GREEN}Нажмите Enter для продолжения...${NC}"
    read
}

# Главный цикл
# Проверяем подключение при старте
if ! detect_auth_method; then
    echo -e "${RED}❌ Не удалось подключиться к MongoDB${NC}"
    echo -e "${YELLOW}Запустите скрипт снова после запуска контейнеров${NC}"
    exit 1
fi

while true; do
    show_menu
    read choice
    
    case $choice in
        1) show_general_metrics ;;
        2) show_connections ;;
        3) show_memory_usage ;;
        4) show_operations ;;
        5) show_collection_stats ;;
        6) show_index_usage ;;
        7) show_active_operations ;;
        8) show_slow_queries ;;
        9) show_replication_status ;;
        10) show_prometheus_metrics ;;
        11) show_full_report ;;
        12) show_realtime_monitor ;;
        13) show_connection_test ;;
        0) 
            echo -e "\n${GREEN}Выход...${NC}"
            exit 0
            ;;
        *)
            echo -e "\n${RED}Неверный выбор. Нажмите Enter...${NC}"
            read
            ;;
    esac
done