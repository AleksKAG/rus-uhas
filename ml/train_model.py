"""
Обучение модели классификации тканей для РУС-УХАС
Использует синтетические данные для демонстрации
В реальности нужны клинические данные с датчиков
"""

import numpy as np
import pandas as pd
from sklearn.ensemble import GradientBoostingClassifier
from sklearn.model_selection import train_test_split
from sklearn.metrics import classification_report, confusion_matrix
from sklearn.preprocessing import StandardScaler
import onnx
from skl2onnx import convert_sklearn
from skl2onnx.common.data_types import FloatTensorType
import matplotlib.pyplot as plt
import seaborn as sns
import os

# Типы тканей (порядок важен!)
TISSUE_TYPES = ['soft', 'vessel', 'nerve', 'bone', 'tumor']
TISSUE_LABELS = {name: i for i, name in enumerate(TISSUE_TYPES)}

def generate_synthetic_data(n_samples=10000, seed=42):
    """
    Генерация синтетических данных на основе клинических диапазонов
    В реальности нужно заменить на реальные данные с датчиков
    """
    np.random.seed(seed)
    
    # Параметры для каждого типа ткани (impedance, temp, power, aspiration, irrigation)
    tissue_params = {
        'soft': {
            'impedance': (55, 15),    # mean, std
            'temp': (38, 2),
            'power': (20, 5),
            'aspiration': (0.3, 0.1),
            'irrigation': (60, 20),
        },
        'vessel': {
            'impedance': (85, 20),
            'temp': (38, 1.5),
            'power': (15, 4),
            'aspiration': (0.25, 0.08),
            'irrigation': (90, 25),
        },
        'nerve': {
            'impedance': (115, 25),
            'temp': (37.5, 1),
            'power': (8, 3),
            'aspiration': (0.15, 0.05),
            'irrigation': (110, 30),
        },
        'bone': {
            'impedance': (250, 60),
            'temp': (40, 3),
            'power': (35, 8),
            'aspiration': (0.5, 0.15),
            'irrigation': (80, 25),
        },
        'tumor': {
            'impedance': (40, 12),
            'temp': (41, 2.5),
            'power': (25, 6),
            'aspiration': (0.45, 0.12),
            'irrigation': (70, 22),
        },
    }
    
    X, y = [], []
    samples_per_class = n_samples // len(TISSUE_TYPES)
    
    for tissue_name, params in tissue_params.items():
        # Генерируем данные для этого типа ткани
        impedance = np.random.normal(params['impedance'][0], params['impedance'][1], samples_per_class)
        temp = np.random.normal(params['temp'][0], params['temp'][1], samples_per_class)
        power = np.random.normal(params['power'][0], params['power'][1], samples_per_class)
        aspiration = np.random.normal(params['aspiration'][0], params['aspiration'][1], samples_per_class)
        irrigation = np.random.normal(params['irrigation'][0], params['irrigation'][1], samples_per_class)
        
        # Ограничиваем диапазоны (физические ограничения датчиков)
        impedance = np.clip(impedance, 10, 500)
        temp = np.clip(temp, 30, 60)
        power = np.clip(power, 0, 50)
        aspiration = np.clip(aspiration, 0, 1)
        irrigation = np.clip(irrigation, 0, 200)
        
        # Добавляем шум (симуляция реальных условий)
        impedance += np.random.normal(0, 5, samples_per_class)
        temp += np.random.normal(0, 0.5, samples_per_class)
        
        X_tissue = np.column_stack([impedance, temp, power, aspiration, irrigation])
        y_tissue = np.full(samples_per_class, TISSUE_LABELS[tissue_name])
        
        X.append(X_tissue)
        y.append(y_tissue)
    
    X = np.vstack(X)
    y = np.concatenate(y)
    
    # Перемешиваем
    indices = np.random.permutation(len(y))
    return X[indices], y[indices]

def train_model(X, y):
    """Обучение модели Gradient Boosting"""
    # Разделяем на train/test
    X_train, X_test, y_train, y_test = train_test_split(
        X, y, test_size=0.2, random_state=42, stratify=y
    )
    
    # Нормализация
    scaler = StandardScaler()
    X_train_scaled = scaler.fit_transform(X_train)
    X_test_scaled = scaler.transform(X_test)
    
    # Обучаем модель
    model = GradientBoostingClassifier(
        n_estimators=200,
        learning_rate=0.1,
        max_depth=5,
        random_state=42,
        verbose=0
    )
    model.fit(X_train_scaled, y_train)
    
    # Оценка
    y_pred = model.predict(X_test_scaled)
    print("\n=== Classification Report ===")
    print(classification_report(y_test, y_pred, target_names=TISSUE_TYPES))
    
    # Confusion matrix
    cm = confusion_matrix(y_test, y_pred)
    plt.figure(figsize=(10, 8))
    sns.heatmap(cm, annot=True, fmt='d', cmap='Blues',
                xticklabels=TISSUE_TYPES, yticklabels=TISSUE_TYPES)
    plt.xlabel('Predicted')
    plt.ylabel('True')
    plt.title('Confusion Matrix')
    plt.savefig('confusion_matrix.png')
    print("\nConfusion matrix saved to confusion_matrix.png")
    
    return model, scaler

def export_to_onnx(model, scaler, output_path='tissue_classifier.onnx'):
    """Экспорт модели в ONNX формат"""
    # Создаем пайплайн: scaler + model
    # Для ONNX нужно объединить их в одну модель
    
    # Создаем входной тензор (5 признаков)
    initial_type = [('float_input', FloatTensorType([None, 5]))]
    
    # Конвертируем sklearn модель в ONNX
    # Примечание: StandardScaler нужно включить в пайплайн
    from sklearn.pipeline import Pipeline
    pipeline = Pipeline([
        ('scaler', scaler),
        ('classifier', model)
    ])
    
    onnx_model = convert_sklearn(
        pipeline,
        initial_types=initial_type,
        target_opset=12,
        options={id(pipeline): {'zipmap': False}}  # Отключаем zipmap для простоты
    )
    
    # Сохраняем
    with open(output_path, 'wb') as f:
        f.write(onnx_model.SerializeToString())
    
    print(f"\nModel exported to {output_path}")
    print(f"Model size: {os.path.getsize(output_path) / 1024:.2f} KB")
    
    # Валидация ONNX модели
    onnx.checker.check_model(onnx_model)
    print("ONNX model validation: OK")
    
    return onnx_model

def extract_scaler_params(scaler):
    """Извлекаем параметры scaler для Go кода"""
    print("\n=== Scaler Parameters (для Go кода) ===")
    print(f"Impedance: mean={scaler.mean_[0]:.2f}, std={np.sqrt(scaler.var_[0]):.2f}")
    print(f"Temp: mean={scaler.mean_[1]:.2f}, std={np.sqrt(scaler.var_[1]):.2f}")
    print(f"Power: mean={scaler.mean_[2]:.2f}, std={np.sqrt(scaler.var_[2]):.2f}")
    print(f"Aspiration: mean={scaler.mean_[3]:.2f}, std={np.sqrt(scaler.var_[3]):.2f}")
    print(f"Irrigation: mean={scaler.mean_[4]:.2f}, std={np.sqrt(scaler.var_[4]):.2f}")

def main():
    print("=== РУС-УХАС: Обучение модели классификации тканей ===\n")
    
    # 1. Генерируем данные
    print("1. Генерация синтетических данных...")
    X, y = generate_synthetic_data(n_samples=10000)
    print(f"   Dataset shape: {X.shape}")
    print(f"   Classes: {np.bincount(y)}")
    
    # 2. Обучаем модель
    print("\n2. Обучение модели...")
    model, scaler = train_model(X, y)
    
    # 3. Экспортируем в ONNX
    print("\n3. Экспорт в ONNX...")
    export_to_onnx(model, scaler, '../models/tissue_classifier.onnx')
    
    # 4. Извлекаем параметры scaler
    extract_scaler_params(scaler)
    
    print("\n=== Готово! ===")
    print("Теперь можно использовать модель в Go через ONNX Runtime")

if __name__ == '__main__':
    main()
