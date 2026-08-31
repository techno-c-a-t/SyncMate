package weather

import (
	"math"
	"sync"
	"time"

	"syncmate/server/internal/domain"
)

type WeatherService struct {
	sync.RWMutex
	cache map[string]*domain.WeatherInfo // key: "lat_lng"
}

func NewWeatherService() *WeatherService {
	return &WeatherService{
		cache: make(map[string]*domain.WeatherInfo),
	}
}

// DistanceInMeters вычисляет расстояние по формуле гаверсинусов между двумя GPS координатами
func DistanceInMeters(lat1, lng1, lat2, lng2 float64) float64 {
	const earthRadiusM = 6371000.0
	dLat := (lat2 - lat1) * math.Pi / 180.0
	dLng := (lng2 - lng1) * math.Pi / 180.0

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180.0)*math.Cos(lat2*math.Pi/180.0)*
			math.Sin(dLng/2)*math.Sin(dLng/2)

	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusM * c
}

// ShouldRefreshWeather проверяет правило: прошло > 20 минут ИЛИ смещение > 1000 метров (1 км)
func ShouldRefreshWeather(lastWeather *domain.WeatherInfo, currentLat, currentLng float64) bool {
	if lastWeather == nil {
		return true
	}

	// 1. Прошел ли 20 минут времени
	if time.Since(lastWeather.FetchedAt) >= 20*time.Minute {
		return true
	}

	// 2. Сместился ли пользователь больше чем на 1000 метров (1 км)
	distM := DistanceInMeters(lastWeather.Lat, lastWeather.Lng, currentLat, currentLng)
	return distM >= 1000.0
}

func (ws *WeatherService) GetWeather(lat, lng float64) *domain.WeatherInfo {
	ws.Lock()
	defer ws.Unlock()

	// Ищем недавний замер в радиусе 1 км
	for _, cached := range ws.cache {
		if !ShouldRefreshWeather(cached, lat, lng) {
			return cached
		}
	}

	// Симуляция получения данных от Weather API (Open-Meteo / OpenWeatherMap)
	// В продакшене здесь делается HTTP запрос к внешнему API
	newWeather := &domain.WeatherInfo{
		Lat:                lat,
		Lng:                lng,
		TemperatureCelsius: 19.5,
		WindSpeedMS:        4.5,   // 4.5 м/с
		WindDirectionDeg:   180.0, // Южный ветер
		FetchedAt:          time.Now(),
	}

	key := fmtKey(lat, lng)
	ws.cache[key] = newWeather
	return newWeather
}

func fmtKey(lat, lng float64) string {
	return string([]byte(string(rune(int(lat*100))) + "_" + string(rune(int(lng*100)))))
}
